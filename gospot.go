package gospot

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/librespot-org/librespot-golang/Spotify"
	"github.com/librespot-org/librespot-golang/librespot"
	"github.com/librespot-org/librespot-golang/librespot/core"
	"github.com/librespot-org/librespot-golang/librespot/utils"
	"gospot/util"
	"io"
	"os"
	"time"
)

var DEBUG bool = false

type FORMAT_TYPE byte

const (
	FORMAT_MP3 FORMAT_TYPE = 1
	FORMAT_OGG FORMAT_TYPE = 2
)

type Localstore struct {
	Username   string
	Authblob   []byte
	DeviceName string
}

type Session struct {
	Ls   *Localstore
	Sess *core.Session
}

type Track struct {
	Id     string
	STrack *Spotify.Track
}

func Login(confFile string, debug bool) (Session, error) {
	DEBUG = debug
	var dirty bool
	var ses Session
	//create config file if it doesnt exist else load config
	f, err := os.OpenFile(confFile, os.O_CREATE|os.O_RDWR, 0600)
	util.CrashAndBurn(err)
	var c []byte = make([]byte, util.FileSize(confFile))
	_, err = f.Read(c)
	util.CrashAndBurn(err)
	//beware trailing commas
	err = json.Unmarshal(c, &ses.Ls)
	util.CrashAndBurn(err)
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
	if ses.Ls.Authblob == nil {
		pass := string(util.PasswdInterrogate("Password (won't echo): "))
		fmt.Println("")
		sess, err := librespot.Login(ses.Ls.Username, pass, ses.Ls.DeviceName)
		pass = ""
		util.CrashAndBurn(err)
		ses.Ls.Authblob = sess.ReusableAuthBlob()
		util.CommitConfig(ses.Ls, f)
	} else {
		ses.Sess, err = librespot.LoginSaved(ses.Ls.Username, ses.Ls.Authblob, ses.Ls.DeviceName)
		util.CrashAndBurn(err)
		if dirty {
			util.CommitConfig(ses.Ls, f)
		}
	}
	//TODO: add actual error handling besides just crashing
	return ses, err
}

func (s Session) GetTrack(trackId string) (Track, error) {
	var err error
	var t Track
	t.Id = trackId
	t.STrack, err = s.Sess.Mercury().GetTrack(utils.Base62ToHex(trackId))
	return t, err
}

// GetAudio returns the audio file in the desired format(mp3 or ogg), starting at high bitrate then going lower if not available. Throws error if audio of type cannot be found.
func (s Session) GetAudio(t Track, format FORMAT_TYPE) ([]byte, error) {
	var fmts []int32
	var auds []*Spotify.AudioFile
	var aud *Spotify.AudioFile
	if t.STrack.GetFile() == nil {
		if DEBUG {
			fmt.Println(t.Id + ": ")
			util.PrintStruct(t)
		}
		auds = t.STrack.Alternative[0].GetFile()
	} else {
		auds = t.STrack.GetFile()
	}

	//set desired format order
	if format == FORMAT_MP3 {
		//mp3: 320kbps, 256kbps, 160kbps, 96kbps
		fmts = []int32{4, 3, 5, 6}
	} else if format == FORMAT_OGG {
		//ogg: 320kbps, 160kbps, 96kbps
		fmts = []int32{3, 2, 1}
	} else {
		return nil, errors.New("invalid format passed")
	}

	for _, i := range fmts {
		for _, tr := range auds {
			//TODO: learn why the cast is necessary
			if int32(*(tr.Format)) == i {
				aud = tr
			}
		}
	}
	if aud == nil {
		return nil, errors.New("no audio found in desired format")
	} else {
		if DEBUG {
			util.PrintStruct(aud)
		}
	}

	decaud, err := s.Sess.Player().LoadTrack(aud, t.STrack.GetGid())
	util.CrashAndBurn(err)
	for decaud.Size() <= 32768 {
		//sleep for 10ms
		time.Sleep(1e7)
	}
	//fmt.Println("size: " + string(decaud.Size()) + " bytes")
	var ogg []byte = make([]byte, decaud.Size())
	var cnk []byte = make([]byte, 128*1024)

	var pos int = 0
	for {
		tf, err := decaud.Read(cnk)
		if err == io.EOF {
			break
		}
		if tf > 0 {
			copy(ogg[pos:pos+tf], cnk[:tf])
			pos += tf
		}
	}
	//TODO: for the love of god do actual error handling
	err = nil
	return ogg, err
}
