package gospot

import (
	"encoding/hex"
	"fmt"
	"github.com/librespot-org/librespot-golang/Spotify"
	"io"
	"io/ioutil"
	"net/http"
)

type TrackStub struct {
	Id     string
	STrack *Spotify.Track
}

type Track struct {
	Title   string
	Artists []string
	Album   string
	Number  int

	Art   Image `json:"-"`
	Sound Audio `json:"-"`
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
	//image is in JPEG format
	res, err := http.Get("https://i.scdn.co/image/" + hex.EncodeToString(i.Stub.FileId))
	if err != nil {
		return i, err
	}
	defer func(Body io.ReadCloser) {
		var err = Body.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(res.Body)
	i.File, err = ioutil.ReadAll(res.Body)
	return i, err
}
