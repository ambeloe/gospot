package util

import (
	"bufio"
	"encoding/json"
	"fmt"
	"golang.org/x/term"
	"log"
	"os"
	"strings"
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
	goddamnitGo, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		fmt.Println(err)
	}
	return goddamnitGo
}

func URLStrip(s string) string {
	var x, y int

	x = strings.LastIndex(s, "/")
	if x == -1 {
		x = 0
	} else {
		x++
	}
	y = strings.Index(s, "?")
	if y == -1 {
		y = len(s)
	}

	return s[x:y]
}

func URIStrip(uri string) string {
	return uri[strings.LastIndex(uri, ":")+1:]
}

func Nop(v interface{}) {}
