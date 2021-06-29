package gospot

import (
	"autotrader/crypt"
	"encoding/json"
	"fmt"
	"github.com/librespot-org/librespot-golang/Spotify"
	_ "github.com/librespot-org/librespot-golang/Spotify"
	"github.com/librespot-org/librespot-golang/librespot"
	_ "github.com/librespot-org/librespot-golang/librespot"
	"github.com/librespot-org/librespot-golang/librespot/core"
	_ "github.com/librespot-org/librespot-golang/librespot/core"
	"github.com/librespot-org/librespot-golang/librespot/utils"
	_ "github.com/librespot-org/librespot-golang/librespot/utils"
	"gospot/util"
	"io"
	"os"
	"time"
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

func Login(confFile string) (Session, error) {
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
		pass := string(crypt.PasswdInterrogate("Password (won't echo): "))
		fmt.Println("")
		sess, err := librespot.Login(ses.Ls.Username, pass, ses.Ls.DeviceName)
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

func (s Session) GetAudio(trackId string, format Spotify.AudioFile_Format) ([]byte, error) {
	trk, err := s.Sess.Mercury().GetTrack(utils.Base62ToHex(trackId))
	util.CrashAndBurn(err)
	var auds []*Spotify.AudioFile
	var aud *Spotify.AudioFile
	if trk.GetFile() == nil {
		auds = trk.Alternative[0].GetFile()
	} else {
		auds = trk.GetFile()
	}
	for _, t := range auds {
		if *(t.Format) == format {
			aud = t
		}
	}
	if aud == nil {
		fmt.Println("shit")
	} else {
		fmt.Println(aud)
	}

	decaud, err := s.Sess.Player().LoadTrack(aud, trk.GetGid())
	util.CrashAndBurn(err)
	fmt.Println(decaud)
	for decaud.Size() <= 32768 {
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
	return ogg, err
}
