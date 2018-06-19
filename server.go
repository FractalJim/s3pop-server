//RFC https://tools.ietf.org/html/rfc1939
package main

/*
    s3pop-server: An AWS S3 backed POP3 server
	Copyright (C) 2018 James W Matheson
	fractal.mango@gmail.com

    This program is free software: you can redistribute it and/or modify
    it under the terms of the GNU Affero General Public License as
    published by the Free Software Foundation, either version 3 of the
    License, or (at your option) any later version.

    This program is distributed in the hope that it will be useful,
    but WITHOUT ANY WARRANTY; without even the implied warranty of
    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
    GNU Affero General Public License for more details.

    You should have received a copy of the GNU Affero General Public License
    along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"

	"github.com/FractalJim/s3pop-server/backend"
	"github.com/FractalJim/s3pop-server/mailutils"
)

const (
	STATE_UNAUTHORIZED = 1
	STATE_TRANSACTION  = 2
	STATE_UPDATE       = 3
)

const eol = "\r\n"
const multilineTerminator = ".\r\n"
const defaultport = 5110

type ServerConfig struct {
	Port     int    `json:"port"`
	S3Bucket string `json:"s3Bucket`
}

func main() {
	config := loadConfig()

	listener, err := net.Listen("tcp", ":"+strconv.Itoa(config.Port))

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error.. %s", err.Error())
	}
	fmt.Println("Server started.")
	fmt.Println("Listening on port: " + strconv.Itoa(config.Port))
	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		// run as goroutine
		go handleClient(conn, config)
	}

}

func loadConfig() (config *ServerConfig) {
	configFilename := "server-config.json"
	config = new(ServerConfig)
	config.Port = defaultport
	jsonData, err := ioutil.ReadFile(configFilename)
	if nil != err {
		log.Fatal("No server-config.json found")
	} else {
		err = json.Unmarshal(jsonData, config)
		if nil != err {
			log.Fatal("Config file is not valid JSON")
		}
	}
	return
}

func handleClient(conn net.Conn, config *ServerConfig) {
	defer conn.Close()

	var state = STATE_UNAUTHORIZED
	var emailDir string
	var emailBucket = config.S3Bucket
	var deletedItems map[int]struct{}
	var mailData []*mailutils.MailData
	reader := bufio.NewReader(conn)

	fmt.Fprintf(conn, "+OK S3 POP3 server: powered by Go"+eol)

	for {
		// Reads a line from the client
		raw_line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Error!!" + err.Error())
			return
		}

		// Parses the command
		cmd, args := getCommand(raw_line)

		fmt.Println("Recieved Command: " + cmd)
		err = nil
		argNum := 0
		var arg string
		for err == nil {
			arg, err = getSafeArg(args, argNum)
			if nil == err {
				fmt.Printf("Argument %d: %s\n", argNum, arg)
			}
			argNum++
		}
		fmt.Println("")
		if cmd == "USER" && state == STATE_UNAUTHORIZED {
			//User name is name of folder in bucket in S3
			userName, err := getSafeArg(args, 0)
			if nil != err {
				writeErrResponse(conn, "No user name", false)
				continue
			}
			emailDir = mailutils.GetEmailDir(userName)
			err = backend.DownloadEmails(emailBucket, userName)
			if nil != err {
				writeErrResponse(conn, "Could not download emails: %s", false, err)
				continue
			}
			mailData = getMessageData(emailDir)
			writeOKResponse(conn, "", true)

		} else if cmd == "PASS" && state == STATE_UNAUTHORIZED {
			//Accept all passwords (local servoce only)
			writeOKResponse(conn, "User signed in", true)
			deletedItems = make(map[int]struct{})
			state = STATE_TRANSACTION

		} else if cmd == "STAT" && state == STATE_TRANSACTION {
			count, size := getStat(mailData, deletedItems)
			writeOKResponse(conn, strconv.Itoa(count)+" "+strconv.Itoa(size), true)

		} else if cmd == "LIST" && state == STATE_TRANSACTION {
			msgId, err := getSafeArg(args, 0)
			if err == nil {
				var id int
				id, _ = strconv.Atoi(msgId)
				id--
				if len(mailData) <= id {
					writeErrResponse(conn, "no such message", false)
					continue
				} else {
					if _, toDel := deletedItems[id]; toDel {
						writeErrResponse(conn, "message deleted", false)
						continue
					}
					writeOKResponse(conn, "%d %d", false, id+1, mailData[id].TotalSize)
				}
			} else {
				count, size := getStat(mailData, deletedItems)
				writeOKResponse(conn, "%d messages (%d octets)", false, count, size)

				for itemId, mailItem := range mailData {
					if _, toDel := deletedItems[itemId]; toDel {
						continue
					}
					fmt.Fprintf(conn, "%d %d\r\n", itemId+1, mailItem.TotalSize)
				}
				fmt.Fprintf(conn, multilineTerminator)
			}

		} else if cmd == "UIDL" && state == STATE_TRANSACTION {

			msgId, err := getSafeArg(args, 0)
			var id int

			if err == nil {
				id, _ = strconv.Atoi(msgId)
				id--
				if len(mailData) <= id {
					writeErrResponse(conn, "no such message", false)
					continue
				} else {
					if _, toDel := deletedItems[id]; toDel {
						writeErrResponse(conn, "message deleted", false)
						continue
					}
					writeOKResponse(conn, "%d %s", false, id+1, mailData[id].Name)
				}
			} else {
				writeOKResponse(conn, "", false)

				for id, mailItem := range mailData {
					if _, toDel := deletedItems[id]; toDel {
						continue
					}
					fmt.Fprintf(conn, "%d %s\r\n", id+1, mailItem.Name)
				}
				fmt.Fprintf(conn, multilineTerminator)
			}

		} else if cmd == "TOP" && state == STATE_TRANSACTION {
			msgId, err := getSafeArg(args, 0)
			var id int

			if err == nil {
				id, _ = strconv.Atoi(msgId)
				id--
				if len(mailData) <= id {
					writeErrResponse(conn, "no such message", false)
					continue
				}
				if _, toDel := deletedItems[id]; toDel {
					writeErrResponse(conn, "message deleted", false)
					continue
				}
			} else {
				writeErrResponse(conn, "no message selected", false)
				continue
			}
			lineArg, err := getSafeArg(args, 1)
			var lines int
			if nil != err {
				writeErrResponse(conn, "no line argument supplied", false)
				continue
			}
			lines, _ = strconv.Atoi(lineArg)

			fullFilePath := filepath.Join(emailDir, mailData[id].Name)
			fileData, err := os.Open(fullFilePath)
			if err != nil {
				writeErrResponse(conn, "failed to open email %s", false, mailData[id].Name)
			}
			defer fileData.Close()
			writeOKResponse(conn, "%d octets", false, mailData[id].TotalSize)
			bodyLinesRead := 0
			inBody := false
			fileScanner := bufio.NewScanner(fileData)
			for fileScanner.Scan() {
				line := fileScanner.Text()
				if line == "" && !inBody {
					fmt.Fprintf(conn, line+eol)
					inBody = true
				} else if line == "." {
					fmt.Fprintf(conn, eol+line+eol)
				} else {
					if inBody {
						bodyLinesRead++
						if bodyLinesRead > lines {
							break
						}
					}
					fmt.Fprintf(conn, line+eol)
				}

			}
			fmt.Fprintf(conn, multilineTerminator)
			fileData.Close()

		} else if cmd == "RETR" && state == STATE_TRANSACTION {
			msgId, err := getSafeArg(args, 0)
			var id int
			if err == nil {
				id, _ = strconv.Atoi(msgId)
				id--
				if len(mailData) <= id {
					writeErrResponse(conn, "no such message", false)
					continue
				}
				if _, toDel := deletedItems[id]; toDel {
					writeErrResponse(conn, "message deleted", false)
					continue
				}
			} else {
				writeErrResponse(conn, "no message selected", false)
				continue
			}

			fullFilePath := filepath.Join(emailDir, mailData[id].Name)
			fileData, err := os.Open(fullFilePath)
			if err != nil {
				writeErrResponse(conn, "failed to open email %s", false, mailData[id].Name)
			}
			defer fileData.Close()
			writeOKResponse(conn, "%d octets", false, mailData[id].TotalSize)

			fileScanner := bufio.NewScanner(fileData)
			for fileScanner.Scan() {
				line := fileScanner.Text()
				if line == "." {
					fmt.Fprintf(conn, eol+line+eol)
				} else {
					fmt.Fprintf(conn, line+eol)
				}

			}
			fmt.Fprintf(conn, multilineTerminator)
			fileData.Close()

		} else if cmd == "DELE" && state == STATE_TRANSACTION {
			msgId, err := getSafeArg(args, 0)
			var id int
			if err == nil {
				id, _ = strconv.Atoi(msgId)
				id--
				if len(mailData) <= id {
					writeErrResponse(conn, "no such message", false)
					continue
				}
				if _, toDel := deletedItems[id]; toDel {
					writeErrResponse(conn, "message already deleted", false)
					continue
				}
			} else {
				writeErrResponse(conn, "no message selected", false)
				continue
			}
			deletedItems[id] = struct{}{}
			fmt.Fprintf(conn, "+OK"+eol)
		} else if cmd == "RSET" {
			deletedItems = make(map[int]struct{})
			writeOKResponse(conn, "", false)
		} else if cmd == "NOOP" {
			writeOKResponse(conn, "", false)
		} else if cmd == "QUIT" {
			if state == STATE_TRANSACTION {
				state = STATE_UPDATE
				deleteItems(emailDir, mailData, deletedItems)
			}
			return
		} else {
			writeErrResponse(conn, "Unrecognised Command", true)
		}
	}
}
