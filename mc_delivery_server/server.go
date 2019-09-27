package main

import (
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"flag"
	"log"
	"gotls"
	"message"
	"networking"
	loadconf "shared/loadconf"
	"net"
	"os"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	daemon "github.com/sevlyar/go-daemon"
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
)

var tlsConfig *tls.Config
var setTimeout = 3 * time.Second

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	flag.Parse()
	daemon.AddCommand(daemon.StringFlag(signal, "quit"), syscall.SIGQUIT, termHandler)
	daemon.AddCommand(daemon.StringFlag(signal, "stop"), syscall.SIGTERM, termHandler)
	daemon.AddCommand(daemon.StringFlag(signal, "reload"), syscall.SIGHUP, reloadHandler)
	// daemon.AddCommand(daemon.StringFlag(signal, "restart"), syscall.SIGUSR2, restartHandler)

	loadconf.SetConfigController()
	dbUser := loadconf.ConfigInfo.DBInfo.DBUser
	dbPassword := loadconf.ConfigInfo.DBInfo.DBPassword
	dbName := loadconf.ConfigInfo.DBInfo.DBName
	dbInfo = "user=" + dbUser + " password=" + dbPassword + " dbname=" + dbName

	tlsConfig = networking.SetTlsConfig()

	cntxt := &daemon.Context{
		PidFileName: "./config/daemon_server.pid",
		PidFilePerm: 0644,
		LogFileName: "./config/daemon_server.log",
		LogFilePerm: 0640,
		WorkDir:     "./",
		Umask:       027,
		Args:        []string{"[mc_delivery_server_daemon]"},
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

		go ListeningFromClient()

		// End of main operation

		err = daemon.ServeSignals()
		if err != nil {
			log.Println("Error:", err)
		}

		log.Println("daemon terminated")
	}

}

var (
	stop       = make(chan struct{})
	done       = make(chan struct{})
	clientStop = make(chan struct{})
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

func ListeningFromClient() {

	defer func() {
		if err := recover(); err != nil {
			log.Println("work failed!: ", err)

			db_err := DBPostgres.Ping()
			if db_err != nil {
				log.Println("error in postgres db: ", db_err)
				log.Println("Will resume listening on port in 10 seconds.")
				time.Sleep(10 * time.Second)
			}
			go ListeningFromClient()
		}
	}()

	listener, err := tls.Listen("tcp", loadconf.ConfigInfo.IPInfo.Port, tlsConfig)
	if err != nil {
		log.Println("listening on port failed: ", err)
	}
	defer listener.Close()

LOOP:
	for {
		select {
		case <-stop:
			log.Println("LOOP from ListeningFromClient")
			break LOOP
		default:

			log.Println("Waiting for clients")

			connFromClient, err := listener.Accept()
			log.Println("Connected: ", connFromClient.RemoteAddr())

			if err != nil {
				log.Println("Client error: ", err)

			} else {
				go ClientHandler(connFromClient)

			}
			defer connFromClient.Close()

		}

	}
	done <- struct{}{}

}

func ClientHandler(conn net.Conn) {

	defer conn.Close()

	mcProtocol := &gotls.McProtocol{}
	conn.SetDeadline(time.Now().Add(setTimeout))

CLIENTLOOP:
	for {
		select {
		case <-clientStop:
			log.Println("log from ClientHandler")
			break CLIENTLOOP
		default:

			p, err := mcProtocol.ReadPacket(conn)
			if err == nil {
				mcPacket := p.(*gotls.McPacket)
				respBody := string(mcPacket.GetBody())
				// respJs := mcPacket.GetBody()
				log.Printf("Message received: %v --> %v\n", mcPacket.GetLength(), respBody)
				conn.SetDeadline(time.Now().Add(setTimeout))
				pBytes := mcPacket.GetBody()

				resultParams := make(map[string]string)
				err2 := json.Unmarshal(pBytes, &resultParams)
				if err2 != nil {
					log.Println(err2)
				}
				queryString2 := message.MapInsertPostgres(resultParams)

				_, err := DBPostgres.Query(queryString2)
				var req []byte
				if err != nil {
					log.Println(err)
					req = []byte(err.Error())

				} else {
					req = []byte("Success")
				}

				i, err2 := conn.Write(gotls.NewMcPacket(req, false).Serialize())
				log.Println(i)
				log.Println(err2)

				log.Println("Keep listening...")
			} else {
				log.Println("Message error: ", err)
				break CLIENTLOOP
			}

		}

	}
	log.Println("Ending CLIENTLOOP...")
}

func checkError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func termHandler(sig os.Signal) error {
	log.Println("terminating...")
	defer DBPostgres.Close()
	clientStop <- struct{}{}
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
