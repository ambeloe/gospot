package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/ambeloe/cliui"
	"github.com/ambeloe/gospot"
	"github.com/ambeloe/gospot/util"
	"github.com/faiface/beep"
	"github.com/faiface/beep/speaker"
	"github.com/faiface/beep/vorbis"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

var ses gospot.Session

func main() {
	var err error

	var conf = flag.String("config", "conf.json", "path for config file")
	var debug = flag.Bool("debug", false, "debug prints")
	flag.Parse()

	ses, err = gospot.Login(*conf, *debug)
	if err != nil {
		fmt.Println("error logging in:", err)
		os.Exit(1)
	}

	var mainmenu cliui.UI
	mainmenu.Name = "player"
	mainmenu.Add("info", info)
	mainmenu.Add("get", get)
	mainmenu.Add("play", play)

	for {
		mainmenu.Run()
	}
}

func info(s []string) {
	if len(s) == 0 {
		fmt.Println("not enough arguments.")
		return
	}
	for i, ss := range s {
		t, err := ses.GetTrack(ss)
		if i != 0 {
			fmt.Println("########")
		}
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println("\tName: " + *t.STrack.Name)
		fmt.Print("\tArtist: ")
		for i, e := range t.STrack.Artist {
			fmt.Print(*e.Name)
			if i < len(t.STrack.Artist)-1 {
				fmt.Print(", ")
			}
		}
		fmt.Println("\n\tAlbum: " + *t.STrack.Album.Name)
		fmt.Println("\tLength: " + strconv.FormatInt(int64(*t.STrack.Duration), 10))
	}
}

//func play_playlist(s []string) {
//	if len(s) != 1 {
//		fmt.Println("playlist only takes one argument")
//		return
//	}
//	s[0] = util.URLStrip(s[0])
//	ses.Sess.Mercury().GetPlaylist(s[0])
//
//}

func get(s []string) {
	if len(s) == 0 {
		fmt.Println("not enough arguments.")
		return
	}

	var t gospot.TrackStub
	var img gospot.Image
	var res []byte
	var err error
	var metapath string

	for i, ss := range s {
		fmt.Println("Getting track ", strconv.Itoa(i))
		metapath = filepath.Join("meta", ss, "song.json")
		res, err = ioutil.ReadFile(metapath)
		if err != nil {
			t, err = ses.GetTrack(ss)
			if err != nil {
				fmt.Println(err)
				return
			}
			//marshal song metadata into json and cache
			res, err = json.Marshal(t)
			util.CrashAndBurn(err)
			err = os.MkdirAll(filepath.Join("meta", ss), os.ModePerm)
			util.CrashAndBurn(err)
			err = ioutil.WriteFile(metapath, res, 0644)
			util.CrashAndBurn(err)

			//get song image and write to disk
			img, err = t.GetImage()
			util.CrashAndBurn(err)
			err = ioutil.WriteFile(filepath.Join("meta", ss, "image.jpg"), img.File, 0644)
			util.CrashAndBurn(err)
		} else {
			err = json.Unmarshal(res, &t)
			util.CrashAndBurn(err)
		}

		err = os.MkdirAll(filepath.Join("audio", ss), os.ModePerm)
		util.CrashAndBurn(err)
		f, err := ses.GetAudio(t, gospot.FormatOgg)
		util.CrashAndBurn(err)
		fp, err := os.OpenFile(filepath.Join("audio", ss, "audio.ogg"), os.O_CREATE|os.O_RDWR, 0644)
		util.CrashAndBurn(err)
		wp, err := fp.Write(*f.File)
		util.CrashAndBurn(err)
		if wp != len(*f.File) {
			fmt.Println("file write error")
		}
	}
}

func play(s []string) {
	var songid = util.URLStrip(s[0])
	f, err := os.Open("audio/" + songid + "/audio.ogg")
	if err != nil {
		get([]string{songid})
	} else {
		fmt.Println("using cached audio")
	}
	f, err = os.Open("audio/" + songid + "/audio.ogg")
	if err != nil {
		fmt.Println("error opening audio")
		return
	}
	str, form, err := vorbis.Decode(f)
	if err != nil {
		fmt.Println("error decoding audio")
		return
	}
	defer str.Close()
	err = speaker.Init(form.SampleRate, form.SampleRate.N(time.Second/10)) //=init speaker and buffer for 100ms
	if err != nil {
		fmt.Println("error opening speaker")
		return
	}
	var fin = make(chan bool)
	fmt.Println("playing...")
	speaker.Play(beep.Seq(str, beep.Callback(func() { fin <- true })))
	<-fin
}
