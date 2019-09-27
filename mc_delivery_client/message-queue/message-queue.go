package messagequeue

import (
	"bufio"
	"bytes"
	"log"
	"os"
	"strings"
	"syscall"

	_ "github.com/lib/pq"
	"github.com/siadat/ipc"
)

var MCDELIVERYROOTDIR string

func GetMyQID() uint64 {

	key, err := ipc.Ftok("/dev/null", 42)
	if err != nil {
		panic(err)
	}

	// log.Println(key)

	qid, err := ipc.Msgget(key, ipc.IPC_CREAT|ipc.IPC_EXCL|0600)
	if err == syscall.EEXIST {
		log.Printf("queue(key=0x%x) exists", key)
		qid, err = ipc.Msgget(key, ipc.IPC_PRIVATE|ipc.IPC_NOWAIT)

	}
	if err != nil {
		log.Println(err)
	}

	return qid
}

func GetConfMap() (map[string]string, error) {

	// var gopath = os.Getenv("GOPATH")
	var confFileName = "../config/columns.conf"

	newConfMap := make(map[string]string)

	file, err := os.Open(confFileName)
	if err != nil {
		return newConfMap, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		a := strings.Split(scanner.Text(), "=")
		b := strings.TrimSpace(a[1])
		c := strings.TrimSpace(a[0])
		newConfMap[c] = b
	}

	if err := scanner.Err(); err != nil {
		return newConfMap, err
	}

	return newConfMap, err

}

func GetConfArray(table string) ([]string, error) {
	if MCDELIVERYROOTDIR = os.Getenv("ROOTDIR_MCDELIVERY"); MCDELIVERYROOTDIR == "" {
		log.Println("[ERR]File not found : config.json. Please check your environment variables. [ROOTDIR_MCDELIVERY]")
		// Exit Code 1 : Configuration File Read Error
		os.Exit(1)
	}

	confPath := MCDELIVERYROOTDIR + "/config/columns.conf"

	// var gopath = os.Getenv("GOPATH")
	// var confFileName = "../config/columns.conf"

	var newConfArray []string

	file, err := os.Open(confPath)
	if err != nil {
		return newConfArray, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		a := strings.Split(scanner.Text(), "=")
		b := strings.TrimSpace(a[1])
		c := strings.TrimSpace(a[0])

		if c == table {
			newConfArray = append(newConfArray, b)
		}

	}

	if err := scanner.Err(); err != nil {
		return newConfArray, err
	}

	return newConfArray, err

}

func GetIDAndTableName(msgRecv []byte) ([]byte, []byte) {

	commaByte := []byte{','}

	splitMsgByte := bytes.Split(msgRecv, commaByte)

	ID := splitMsgByte[0]

	var table []byte

	if len(splitMsgByte) > 2 {
		splitMsgByte = splitMsgByte[1:]
		table = bytes.Join(splitMsgByte, commaByte)
		table = bytes.TrimSpace(table)

	} else {
		table = splitMsgByte[1]
		table = bytes.TrimSpace(table)
		// log.Println(table)
	}

	return ID, table

}
