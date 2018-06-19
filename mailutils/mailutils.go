package mailutils

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
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path/filepath"
)

type MailData struct {
	ID          int    `json:"id"`
	HeaderSize  int    `json:"headerSize"`
	MessageSize int    `json:"messageSize"`
	TotalSize   int    `json:"totalSize"`
	Read        bool   `json:"read"`
	Name        string `json:"name"`
}

func (m *MailData) Save(emailDir string) {
	jsonData, err := json.Marshal(&m)
	checkError(err)

	metadataFilename := filepath.Join(emailDir, m.Name+".json")
	metadataFile, err := os.Create(metadataFilename)
	checkError(err)
	defer metadataFile.Close()

	metadataFile.Write(jsonData)
}

func LoadMailData(emailDir string, filename string) (m *MailData) {
	if filepath.Ext(filename) != ".json" {
		filename += ".json"
	}
	metadataFilename := filepath.Join(emailDir, filename)
	jsonData, err := ioutil.ReadFile(metadataFilename)
	checkError(err)

	m = &MailData{Read: false}
	err = json.Unmarshal(jsonData, m)
	checkError(err)
	return
}

func GetEmailDir(emailUser string) string {
	var userInfo *user.User
	userInfo, err := user.Current()
	checkError(err)

	dirName := filepath.Join(userInfo.HomeDir, ".email")
	_, err = os.Stat(dirName)
	if nil != err {
		err = os.Mkdir(dirName, 0700)
		checkError(err)
	}
	emailPath := filepath.Join(dirName, emailUser)
	_, err = os.Stat(emailPath)
	if nil != err {
		err = os.Mkdir(emailPath, 0700)
		checkError(err)
	}
	return emailPath
}

func checkError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
