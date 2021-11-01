package gospot

import (
	"encoding/json"
	"fmt"
	"github.com/ambeloe/gospot/util"
	"github.com/librespot-org/librespot-golang/Spotify"
	"github.com/librespot-org/librespot-golang/librespot"
	"github.com/librespot-org/librespot-golang/librespot/metadata"
	"os"
	"time"
)

var DEBUG = false

type FormatType byte

const (
	FormatMp3 FormatType = 1
	FormatOgg FormatType = 2
)

type LocalStore struct {
	Username   string
	AuthBlob   []byte
	DeviceName string

	OauthToken OToken
}

type OToken struct {
	IssueTime time.Time
	Token     *metadata.Token
}

type Image struct {
	Stub *Spotify.Image
	File []byte
}

type Audio struct {
	Format Spotify.AudioFile_Format
	File   *[]byte
}

type Playlist struct {
	Id       string
	Name     string
	Thumb    Image
	Len      int
	Checksum []byte
	Songs    []TrackStub
	Stub     *Spotify.Playlist
}

func Login(confFile string, debug bool) (Session, error) {
	DEBUG = debug
	var dirty bool
	var ses Session
	ses.Ls = &LocalStore{
		Username:   "",
		AuthBlob:   nil,
		DeviceName: "",
	}
	//create config file if it doesnt exist else load config
	f, err := os.OpenFile(confFile, os.O_CREATE|os.O_RDWR, 0600)
	util.CrashAndBurn(err)
	var c = make([]byte, util.FileSize(confFile))
	_, err = f.Read(c)
	util.CrashAndBurn(err)
	s, err := f.Stat()
	if s.Size() > 0 {
		//beware trailing commas
		err = json.Unmarshal(c, &ses.Ls)
		util.CrashAndBurn(err)
	}
	if ses.Ls.DeviceName == "" {
		ses.Ls.DeviceName = string(util.Interrogate("Device name: "))
		dirty = true
	} else {
		fmt.Println("Using saved device name: \"" + ses.Ls.DeviceName + "\"")
	}
	if ses.Ls.Username == "" {
		ses.Ls.Username = string(util.Interrogate("Username: "))
		dirty = true
	} else {
		fmt.Println("Using saved username: \"" + ses.Ls.Username + "\"")
	}
	if ses.Ls.AuthBlob == nil {
		pass := string(util.PasswdInterrogate("Password (won't echo): "))
		fmt.Println("")
		sess, err := librespot.Login(ses.Ls.Username, pass, ses.Ls.DeviceName)
		pass = ""
		util.CrashAndBurn(err)
		ses.Ls.AuthBlob = sess.ReusableAuthBlob()
		util.CommitConfig(ses.Ls, f)
	} else {
		ses.Sess, err = librespot.LoginSaved(ses.Ls.Username, ses.Ls.AuthBlob, ses.Ls.DeviceName)
		util.CrashAndBurn(err)
		if dirty {
			util.CommitConfig(ses.Ls, f)
		}
	}
	//TODO: add actual error handling besides just crashing
	return ses, err
}
