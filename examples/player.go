package main

import (
	"flag"
	"fmt"
	"github.com/ambeloe/cliui"
	"github.com/ambeloe/gospot"
	"github.com/ambeloe/gospot/util"
	"github.com/faiface/beep"
	"github.com/faiface/beep/speaker"
	"github.com/faiface/beep/vorbis"
	"os"
	"strconv"
	"time"
)

var ses gospot.Session

func main() {
	var err error

	var conf = flag.String("config", "", "path for config file")
	var debug = flag.Bool("debug", false, "debug prints")
	flag.Parse()

	if *conf != "" {
		ses, err = gospot.Login(*conf, *debug)
		util.CrashAndBurn(err)
	} else {
		ses, err = gospot.Login("conf.json", *debug)
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

func get(s []string) {
	if len(s) == 0 {
		fmt.Println("not enough arguments.")
		return
	}
	for i, ss := range s {
		fmt.Println("Getting track ", strconv.Itoa(i))
		t, err := ses.GetTrack(ss)
		if err != nil {
			fmt.Println(err)
			return
		}
		err = os.MkdirAll("audio/"+ss, os.ModePerm)
		util.CrashAndBurn(err)
		f, err := ses.GetAudio(t, gospot.FormatOgg)
		util.CrashAndBurn(err)
		fp, err := os.OpenFile("audio/"+ss+"/audio.ogg", os.O_CREATE|os.O_RDWR, 0644)
		util.CrashAndBurn(err)
		wp, err := fp.Write(*f.File)
		util.CrashAndBurn(err)
		if wp != len(*f.File) {
			fmt.Println("file write error")
		}
	}
}

func play(s []string) {
	f, err := os.Open("audio/" + s[0] + "/audio.ogg")
	if err != nil {
		get(s[:1])
	} else {
		fmt.Println("using cached audio")
	}
	f, err = os.Open("audio/" + s[0] + "/audio.ogg")
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
