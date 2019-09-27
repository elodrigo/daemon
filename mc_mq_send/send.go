package main

import (
	"log"
	"syscall"

	_ "github.com/lib/pq"
	"github.com/siadat/ipc"
)

func main() {

	SendMQ()

}

func getMyQID() uint64 {

	key, err := ipc.Ftok("/dev/null", 42)
	if err != nil {
		panic(err)
	}

	// log.Println(key)

	qid, err := ipc.Msgget(key, ipc.IPC_CREAT|ipc.IPC_EXCL|0600)
	if err == syscall.EEXIST {
		log.Printf("queue(key=0x%x) exists", key)
		qid, err = ipc.Msgget(key, ipc.IPC_PRIVATE|ipc.IPC_NOWAIT)
		log.Println(qid)

	}
	if err != nil {
		log.Println(err)
	}

	return qid
}

func SendMQ() {

	qid := getMyQID()

	log.Println("Qid is: ", qid)

	msg := &ipc.Msgbuf{Mtype: 12, Mtext: []byte("1, detect_log_target")}
	err3 := ipc.Msgsnd(qid, msg, 0)
	if err3 != nil {
		log.Fatal(err3)
	}

	log.Printf("Sent message: %q", msg.Mtext)

}
