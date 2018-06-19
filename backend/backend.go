package backend

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
	"errors"
	"fmt"
	"log"
	"os"
	"os/user"
	"path"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"

	"github.com/FractalJim/s3pop-server/mailutils"
)

const indexFileName = "_email_index.txt"
const multipartHeader = "Content-Type: multipart"

type mailFile struct {
	index    int
	filename string
}

//index management functions
//index keeps track of ids of all emails ever seen, it is never deleted from
func loadIndex(emailDir string) (filesByIndex map[int]*mailFile, filesByName map[string]*mailFile) {
	filesByIndex = make(map[int]*mailFile)
	filesByName = make(map[string]*mailFile)
	var indexFile = filepath.Join(emailDir, indexFileName)
	_, err := os.Stat(indexFile)
	if nil != err {
		//index does not exist yet or cant be opened
		return
	}

	var indexData *os.File
	indexData, err = os.Open(indexFile)
	checkError(err)
	defer indexData.Close()

	var indexScanner = bufio.NewScanner(indexData)
	var currentIndex int = 0
	for indexScanner.Scan() {
		var thisFile = &mailFile{
			filename: indexScanner.Text(),
			index:    currentIndex,
		}
		filesByIndex[currentIndex] = thisFile
		filesByName[indexScanner.Text()] = thisFile
		currentIndex++
	}

	checkError(indexScanner.Err())
	return
}

func appendIndex(name, emailDir string, filesByIndex map[int]*mailFile, filesByName map[string]*mailFile) {
	var indexFile = filepath.Join(emailDir, indexFileName)
	var indexData *os.File
	_, err := os.Stat(indexFile)
	if nil != err {
		//index does not exist yet or cant be opened
		indexData, err = os.Create(indexFile)
	} else {
		indexData, err = os.OpenFile(indexFile, os.O_APPEND|os.O_WRONLY, 0600)
	}

	checkError(err)
	defer indexData.Close()

	indexData.WriteString(name + "\n")
	var newID = getNextID(filesByIndex)

	var thisFile = &mailFile{
		filename: name,
		index:    newID,
	}
	filesByIndex[newID] = thisFile
	filesByName[name] = thisFile
}

func getNextID(filesByIndex map[int]*mailFile) int {
	var res int
	for key := range filesByIndex {
		if key > res {
			res = key
		}
	}
	return res + 1
}

func DownloadEmails(emailBucket, emailFolder string) error {

	sess, err := getSession()
	if nil != err {
		return err
	}
	svc := s3.New(sess)

	params := &s3.ListObjectsInput{
		Bucket: aws.String(emailBucket),
		Prefix: aws.String(emailFolder),
	}

	resp, err := svc.ListObjects(params)
	if nil != err {
		return err
	}
	userEmailDir := mailutils.GetEmailDir(emailFolder)
	filesByIndex, filesByName := loadIndex(userEmailDir)

	for _, key := range resp.Contents {
		emailId := path.Base(*key.Key)
		_, known := filesByName[emailId]
		if !known {
			nextPopId := getNextID(filesByIndex)
			emailFile := filepath.Join(userEmailDir, emailId)
			err = downloadFile(*key.Key, emailBucket, emailFile, sess)
			if nil != err {
				return err
			}
			processEmail(userEmailDir, emailId, nextPopId)
			appendIndex(emailId, userEmailDir, filesByIndex, filesByName)
		}
	}
	return nil
}

func processEmail(emailDir string, filename string, id int) {
	emailFile := filepath.Join(emailDir, filename)
	headers, body := splitEmail(emailFile)
	headerSize := calcPartSizeBytes(headers)
	bodySize := calcPartSizeBytes(body)
	metadata := &mailutils.MailData{
		Name:        filename,
		ID:          id,
		Read:        false,
		HeaderSize:  headerSize,
		MessageSize: bodySize,
		TotalSize:   headerSize + bodySize,
	}
	metadata.Save(emailDir)
}

func splitEmail(fullFilePath string) (headers []string, body []string) {
	fileData, err := os.Open(fullFilePath)
	checkError(err)
	defer fileData.Close()

	headers = make([]string, 0)
	body = make([]string, 0)
	var inHeaders = true

	fileScanner := bufio.NewScanner(fileData)

	for fileScanner.Scan() {
		if fileScanner.Text() == "" {
			if inHeaders {
				inHeaders = false
			}
		}
		if inHeaders {
			headers = append(headers, fileScanner.Text())
		} else {
			body = append(body, fileScanner.Text())
		}
	}

	return
}

func calcPartSizeBytes(part []string) int {
	var sum int
	for _, line := range part {
		sum += len(line) + 2
	}
	return sum
}

func downloadFile(key, bucket string, outputPath string, sess *session.Session) error {

	fmt.Printf("Beginning download of %s.\n", key)
	file, err := os.Create(outputPath)
	if nil != err {
		return err
	}
	defer file.Close()

	downloader := s3manager.NewDownloader(sess)

	_, err = downloader.Download(file, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if nil != err {
		return err
	}
	file.Close()
	fmt.Printf("Download of %s complete.\n", key)
	fmt.Printf("Downloaded file written to %s.\n", outputPath)

	return err
}

func getSession() (sess *session.Session, err error) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Panic creating Session:", r)
			err = errors.New(r.(string))
		}
	}()
	var userInfo *user.User
	userInfo, err = user.Current()
	if nil != err {
		return nil, err
	}

	_, err = os.Stat(filepath.Join(userInfo.HomeDir, ".aws", "config"))
	if nil != err {
		return nil, err
	}

	_, err = os.Stat(filepath.Join(userInfo.HomeDir, ".aws", "config"))
	if nil != err {
		return nil, err
	}

	sess, err = session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	})
	return
}

func checkError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
