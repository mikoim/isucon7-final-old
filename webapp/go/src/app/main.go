package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/pprof"
	"net/url"
	"os"
	"time"
	"math/rand"

	_ "net/http/pprof"
	_ "expvar"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/jmoiron/sqlx"
)

var (
	db *sqlx.DB
	hosts []string
	nyan int64
)

func initDB() {
	db_host := os.Getenv("ISU_DB_HOST")
	if db_host == "" {
		db_host = "127.0.0.1"
	}
	db_port := os.Getenv("ISU_DB_PORT")
	if db_port == "" {
		db_port = "3306"
	}
	db_user := os.Getenv("ISU_DB_USER")
	if db_user == "" {
		db_user = "root"
	}
	db_password := os.Getenv("ISU_DB_PASSWORD")
	if db_password != "" {
		db_password = ":" + db_password
	}

	dsn := fmt.Sprintf("%s%s@tcp(%s:%s)/isudb?parseTime=true&loc=Local&charset=utf8mb4",
		db_user, db_password, db_host, db_port)

	log.Printf("Connecting to db: %q", dsn)
	db, _ = sqlx.Connect("mysql", dsn)
	for {
		err := db.Ping()
		if err == nil {
			break
		}
		log.Println(err)
		time.Sleep(time.Second * 3)
	}

	db.SetMaxOpenConns(20)
	db.SetConnMaxLifetime(5 * time.Minute)
	log.Printf("Succeeded to connect db.")
}

func getInitializeHandler(w http.ResponseWriter, r *http.Request) {
	db.MustExec("TRUNCATE TABLE adding")
	db.MustExec("TRUNCATE TABLE buying")
	db.MustExec("TRUNCATE TABLE room_time")
	w.WriteHeader(204)
}

func getRoomHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	roomName := vars["room_name"]
	path := "/ws/" + url.PathEscape(roomName)

	nyan++

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(struct {
		Host string `json:"host"`
		Path string `json:"path"`
	}{
		Host: hosts[nyan % 3],
		Path: path,
	})
}

func wsGameHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	roomName := vars["room_name"]

	ws, err := websocket.Upgrade(w, r, nil, 1024, 1024)
	if _, ok := err.(websocket.HandshakeError); ok {
		log.Println("Failed to upgrade", err)
		return
	}
	go serveGameConn(ws, roomName)
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	initDB()

	rand.Seed(time.Now().UTC().UnixNano())

	hosts = []string{"app0191.isu7f.k0y.org", "app0192.isu7f.k0y.org","app0193.isu7f.k0y.org"}

	r := mux.NewRouter()

	r.HandleFunc("/debug/pprof/", pprof.Index)
	r.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	r.HandleFunc("/debug/pprof/profile", pprof.Profile)
	r.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	r.HandleFunc("/debug/pprof/trace", pprof.Trace)
	r.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
	r.Handle("/debug/pprof/heap", pprof.Handler("heap"))
	r.Handle("/debug/pprof/threadcreate", pprof.Handler("threadcreate"))
	r.Handle("/debug/pprof/block", pprof.Handler("block"))
	r.Handle("/debug/vars", http.DefaultServeMux)

	r.HandleFunc("/initialize", getInitializeHandler)
	r.HandleFunc("/room/", getRoomHandler)
	r.HandleFunc("/room/{room_name}", getRoomHandler)
	r.HandleFunc("/ws/", wsGameHandler)
	r.HandleFunc("/ws/{room_name}", wsGameHandler)
	r.PathPrefix("/").Handler(http.FileServer(http.Dir("../public/")))

	r.PathPrefix("/debug/pprof/profile").HandlerFunc(pprof.Profile)

	log.Fatal(http.ListenAndServe(":5000", handlers.LoggingHandler(os.Stderr, r)))
}
