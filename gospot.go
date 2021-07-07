package gospot

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gospot/util"
	"github.com/librespot-org/librespot-golang/Spotify"
	"github.com/librespot-org/librespot-golang/librespot"
	"github.com/librespot-org/librespot-golang/librespot/core"
	"github.com/librespot-org/librespot-golang/librespot/utils"
	"io"
	"net/http"
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
}

type Session struct {
	Ls   *LocalStore
	Sess *core.Session
}

type TrackStub struct {
	Id     string
	STrack *Spotify.Track
}

type Track struct {
	Title   string
	Artists []string
	Album   string
	Art     Image
	Sound   Audio
}

type Image struct {
	Stub *Spotify.Image
	File []byte
}

type Audio struct {
	Format Spotify.AudioFile_Format
	File   []byte
}

func Login(confFile string, debug bool) (Session, error) {
	DEBUG = debug
	var dirty bool
	var ses Session
	//create config file if it doesnt exist else load config
	f, err := os.OpenFile(confFile, os.O_CREATE|os.O_RDWR, 0600)
	util.CrashAndBurn(err)
	var c = make([]byte, util.FileSize(confFile))
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

func (s Session) GetTrack(trackId string) (TrackStub, error) {
	var err error
	var t TrackStub
	t.Id = trackId
	t.STrack, err = s.Sess.Mercury().GetTrack(utils.Base62ToHex(trackId))
	return t, err
}

// GetAudio returns the audio file in the desired format(mp3 or ogg), starting at high bitrate then going lower if not available. Throws error if audio of type cannot be found.
func (s Session) GetAudio(t TrackStub, format FormatType) (Audio, error) {
	var a Audio
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
	if format == FormatMp3 {
		//mp3: 320kbps, 256kbps, 160kbps, 96kbps
		fmts = []int32{4, 3, 5, 6}
	} else if format == FormatOgg {
		//ogg: 320kbps, 160kbps, 96kbps
		fmts = []int32{2, 1, 0}
	} else {
		return a, errors.New("invalid format passed")
	}

	for _, i := range fmts {
		for _, tr := range auds {
			//TODO: learn why the cast is necessary
			if int32(*(tr.Format)) == i {
				a.Format = Spotify.AudioFile_Format(i)
				aud = tr
				goto seethe
			}
		}
	}
seethe:
	if aud == nil {
		return a, errors.New("no audio found in desired format")
	} else {
		if DEBUG {
			util.PrintStruct(aud)
		}
	}

	decaud, err := s.Sess.Player().LoadTrack(aud, t.STrack.GetGid())
	util.CrashAndBurn(err)
	//TODO: find out why this size
	for decaud.Size() <= 32768 {
		//sleep for 10ms
		time.Sleep(1e7)
	}
	//fmt.Println("size: " + string(decaud.Size()) + " bytes")
	//a.File, err = io.ReadAll(decaud)
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
	return a, err
}

func (t TrackStub) GetImage() (Image, error) {
	var i Image
	var err error
	//try to find largest possible image
	for s := 3; s > -1; s-- {
		for _, im := range t.STrack.Album.CoverGroup.Image {
			if *(im.Size) == Spotify.Image_Size(s) {
				i.Stub = im
				goto eol
			}
		}
	}
eol:
	res, err := http.Get("https://i.scdn.co/image/" + hex.EncodeToString(i.Stub.FileId))
	if err != nil {
		return i, err
	}
	//TODO: maybe add error handling
	defer res.Body.Close()
	i.File, err = io.ReadAll(res.Body)
	return i, err
}
