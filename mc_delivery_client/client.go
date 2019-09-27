package main

import (
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"flag"
	"log"
	messagequeue "message-queue"
	"networking"
	"networking/gotls"
	loadconf "shared/loadconf"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	daemon "github.com/sevlyar/go-daemon"
	"github.com/siadat/ipc"
)

var (
	signal = flag.String("s", "", `send signal to the daemon
		quit — graceful shutdown
		stop — fast shutdown
		reload — reloading the configuration file
		reborn — restarting child process`)
)

var dbInfo string

var DBPostgres *sql.DB

var (
	dbReady = make(chan struct{})
	// srvReady = make(chan struct{})
	mu sync.Mutex
)

var tlsConfig *tls.Config
var ConnToSrv *tls.Conn

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	flag.Parse()
	daemon.AddCommand(daemon.StringFlag(signal, "quit"), syscall.SIGQUIT, termHandler)
	daemon.AddCommand(daemon.StringFlag(signal, "stop"), syscall.SIGTERM, termHandler)
	daemon.AddCommand(daemon.StringFlag(signal, "reload"), syscall.SIGHUP, reloadHandler)
	daemon.AddCommand(daemon.StringFlag(signal, "restart"), syscall.SIGUSR2, restartHandler)

	loadconf.SetConfigController()
	dbUser := loadconf.ConfigInfo.DBInfo.DBUser
	dbPassword := loadconf.ConfigInfo.DBInfo.DBPassword
	dbName := loadconf.ConfigInfo.DBInfo.DBName
	dbInfo = "user=" + dbUser + " password=" + dbPassword + " dbname=" + dbName

	tlsConfig = networking.SetTlsConfig()

	cntxt := &daemon.Context{
		PidFileName: "./config/daemon_client.pid",
		PidFilePerm: 0644,
		LogFileName: "./config/daemon_client.log",
		LogFilePerm: 0640,
		WorkDir:     "./",
		Umask:       027,
		Args:        []string{"[mc_delivery_client_daemon]"},
	}

	if len(daemon.ActiveFlags()) > 0 {
		d, err := cntxt.Search()
		if err != nil {
			log.Fatalln("Unable to send signal to the daemon:", err)
		}
		daemon.SendCommands(d)
		return
	}

	d, err := cntxt.Reborn()
	if err != nil {
		log.Fatal("Unable to run: ", err)
	}
	if d != nil {

		/*
			go func() {

				fmt.Println("start")
				for {
					time.Sleep(4 * time.Second)
					fmt.Println("parent routine")

				}

				<-stop
				fmt.Println("stop signed")
				return
			}()
		*/
		return

	} else {
		defer cntxt.Release()

		log.Print("- - - - - - - - - - - - - - -")
		log.Print("daemon started")

		// Run main operation
		go connectToPostgres(dbInfo)
		<-dbReady

		go runOperation()

		// End of main operation

		err = daemon.ServeSignals()
		if err != nil {
			log.Println("Error:", err)
		}

		if ConnToSrv != nil {
			ConnToSrv.Close()
		}

		log.Println("daemon terminated")
	}

}

var (
	stop = make(chan struct{})
	done = make(chan struct{})
)

func connectToPostgres(dbInfo string) {
	var err error

	for i := 0; i < 2; i++ {

		DBPostgres, err = sql.Open("postgres", dbInfo)
		if err != nil {
			log.Println("sql connetion failed: ", err)
			continue
		} else {
			log.Println("sql connected")

			dbReady <- struct{}{}
			break
		}

	}
	if err != nil {
		log.Println("Sql connection trial will be restarted in 10 sec.")
		time.Sleep(10 * time.Second)
		go connectToPostgres(dbInfo)
	}

}

func runOperation() {
	connectToServer()
	recvMQ()
}

func reconnectToServer() {
	mu.Lock()
	ConnToSrv.Close()
	mu.Unlock()

	log.Println("Reconnecting to server...")
	time.Sleep(time.Second)

	go runOperation()
}

func connectToServer() {
	var err error

	for i := 0; i < 2; i++ {
		mu.Lock()
		ConnToSrv, err = tls.Dial("tcp", loadconf.ConfigInfo.IPInfo.IP+loadconf.ConfigInfo.IPInfo.Port, tlsConfig)
		mu.Unlock()

		if err != nil {
			log.Println("Connection to server failed: ", err)
			continue
		} else {
			log.Println("Connection to server succeed")
			break
		}

	}
	if err != nil {
		log.Println("Server connection trial will be restarted in 10 sec.")
		time.Sleep(10 * time.Second)
		connectToServer()
	}
}

func recvMQ() {

	qid := messagequeue.GetMyQID()

	msg := &ipc.Msgbuf{Mtype: 12}
	if msg == nil {
		log.Println("nil!")
	}

	defer func() {
		if err := recover(); err != nil {
			log.Println("work failed!: ", err)

			db_err := DBPostgres.Ping()
			if db_err != nil {
				log.Println("error in postgres db: ", db_err)
				log.Println("Will resume message queue receiving procedure in 10 seconds.")
				time.Sleep(10 * time.Second)
				go connectToPostgres(dbInfo)
				<-dbReady
			}
			recvMQ()

		}
	}()

LOOP:
	for {
		select {
		case <-stop:
			break LOOP
		default:

			time.Sleep(300 * time.Millisecond)

			err := ipc.Msgrcv(qid, msg, ipc.IPC_NOWAIT)

			if err != nil {
				// log.Println(err)
			} else {

				queryString, table := messagequeue.GetSelectQueryString(msg.Mtext)

				rows, err := DBPostgres.Query(queryString)
				if err != nil {
					log.Println(err)
				}
				cv := messagequeue.GetQueryResult(rows)
				cv["table_name"] = table

				for key, value := range cv {

					cv[key] = strings.TrimSpace(value)

				}
				defer rows.Close()

				// Send to server
				js, err := json.Marshal(cv)
				if err != nil {
					log.Println("json marshal error: ", err)
				}

				i, err2 := ConnToSrv.Write(gotls.NewMcPacket(js, false).Serialize())
				log.Println(i)
				log.Println(err2)

				// Read from server
				mcProtocol := &gotls.McProtocol{}
				p, errRead := mcProtocol.ReadPacket(ConnToSrv)

				for i := 0; i < 2; i++ {
					if errRead == nil {
						mcPacket := p.(*gotls.McPacket)
						s := string(mcPacket.GetBody())
						log.Printf("Server reply: [%v] [%v]\n", mcPacket.GetLength(), s)
						if s == "Success" {
							break
						} else {
							continue
						}

					} else {
						log.Println(errRead)
						mu.Lock()
						ConnToSrv, err = tls.Dial("tcp", loadconf.ConfigInfo.IPInfo.IP+loadconf.ConfigInfo.IPInfo.Port, tlsConfig)
						mu.Unlock()
						time.Sleep(500 * time.Millisecond)

						ConnToSrv.Write(gotls.NewMcPacket(js, false).Serialize())
						p, errRead = mcProtocol.ReadPacket(ConnToSrv)
						_, _ = p, errRead

					}
				}
				if errRead != nil {
					reconnectToServer()
					break LOOP

				}

			}
		}

	}
	// done <- struct{}{}
}

func termHandler(sig os.Signal) error {
	log.Println("terminating...")
	defer DBPostgres.Close()
	stop <- struct{}{}
	if sig == syscall.SIGQUIT {
		<-done
	}
	return daemon.ErrStop
}

func reloadHandler(sig os.Signal) error {
	log.Println("configuration reloaded")
	return nil
}

func restartHandler(sig os.Signal) error {
	log.Println("Restarting process...")
	return nil
}
