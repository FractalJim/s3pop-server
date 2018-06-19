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
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/FractalJim/s3pop-server/mailutils"
)

func getMessageData(emailDir string) []*mailutils.MailData {
	var emailMetafiles []string
	filepath.Walk(emailDir, func(path string, info os.FileInfo, _ error) error {
		if !info.IsDir() {
			if filepath.Ext(path) == ".json" {
				emailMetafiles = append(emailMetafiles, filepath.Base(path))
			}
		}
		return nil
	})

	var result = make([]*mailutils.MailData, 0)
	for _, mailItem := range emailMetafiles {
		itemDetails := mailutils.LoadMailData(emailDir, mailItem)
		result = append(result, itemDetails)
	}
	return result
}

func getStat(mailData []*mailutils.MailData, deletedItems map[int]struct{}) (count int, size int) {

	count = 0
	for id, mailItem := range mailData {
		if _, toDel := deletedItems[id]; toDel {
			continue
		}
		count++
		size += mailItem.TotalSize
	}
	return
}

func getCommand(line string) (string, []string) {
	line = strings.Trim(line, "\r \n")
	cmd := strings.Split(line, " ")
	return cmd[0], cmd[1:]
}
func getSafeArg(args []string, argIndex int) (string, error) {
	if argIndex < len(args) {
		return args[argIndex], nil
	}
	return "", errors.New("Index out of range")
}

func writeOKResponse(conn net.Conn, msg string, log bool, args ...interface{}) {
	fmt.Fprintf(conn, "+OK "+msg+eol, args...)
	if log {
		fmt.Printf("+OK "+msg, args...)
	}
}

func writeErrResponse(conn net.Conn, msg string, log bool, args ...interface{}) {
	fmt.Fprintf(conn, "-ERR "+msg+eol, args...)
	if log {
		fmt.Printf("-ERR "+msg, args...)
	}
}

func deleteItems(emailDir string, mailData []*mailutils.MailData, deletedItems map[int]struct{}) (removeSucceed int, removeFailed int) {
	for id := range deletedItems {
		filename := filepath.Join(emailDir, mailData[id].Name)
		err := os.Remove(filename + ".json")
		os.Remove(filename)
		if nil == err {
			removeSucceed++
		} else {
			removeFailed++
		}
	}
	return
}
