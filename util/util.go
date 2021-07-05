package util

import (
	"bufio"
	"encoding/json"
	"fmt"
	"golang.org/x/crypto/ssh/terminal"
	"log"
	"os"
	"syscall"
)

func CommitConfig(v interface{}, f *os.File) {
	js, err := json.MarshalIndent(v, "", "\t")
	CrashAndBurn(err)
	err = f.Truncate(0)
	CrashAndBurn(err)
	_, err = f.WriteAt(js, 0)
	CrashAndBurn(err)
}

func Interrogate(q string) []byte {
	var sc = bufio.NewReader(os.Stdin)
	fmt.Print(q)
	str, err := sc.ReadBytes('\n')
	CrashAndBurn(err)
	return str[:len(str)-1]
}

func CrashAndBurn(e error) {
	if e != nil {
		log.Fatal(e)
	}
}

func PrintStruct(v interface{}) {
	str, err := json.MarshalIndent(v, "", "  ")
	CrashAndBurn(err)
	fmt.Println(string(str))
}

func FileSize(path string) int64 {
	f, err := os.Open(path)
	CrashAndBurn(err)
	fi, err := f.Stat()
	CrashAndBurn(err)
	return fi.Size()
}

func PasswdInterrogate(prompt string) []byte {
	fmt.Print(prompt)
	goddamnit_go, err := terminal.ReadPassword(syscall.Stdin)
	if err != nil {
		fmt.Println(err)
	}
	return goddamnit_go
}
