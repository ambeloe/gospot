package gospot

import (
	"errors"
	"github.com/ambeloe/gospot/util"
	"github.com/librespot-org/librespot-golang/Spotify"
	"github.com/librespot-org/librespot-golang/librespot/utils"
)

// ErrMissingImg todo: engooden
var ErrMissingImg = errors.New("missing image")

type TrackStub struct {
	Id     string
	STrack *Spotify.Track `json:",omitempty"`
}

type Track struct {
	Id string

	Title   string
	Artists []Artist
	Album   Album
	Number  int

	Art   Image `json:"-"`
	Sound Audio `json:"-"`

	Stub *Spotify.Track `json:",omitempty"`
}

func (t TrackStub) GetImage() (Image, error) {
	var i Image
	var err error
	//try to find largest possible image
	for s := 3; s > -1; s-- {
		if t.STrack.Album.CoverGroup == nil {
			return i, ErrMissingImg
		}
		for _, im := range t.STrack.Album.CoverGroup.Image {
			if *(im.Size) == Spotify.Image_Size(s) {
				i.Id = im.FileId
				goto eol
			}
		}
	}
eol:
	i.File, err = util.GetImageFile(i.Id)
	return i, err
}

func (t TrackStub) GetArtists() ([]Artist, error) {
	if t.STrack == nil {
		return nil, errors.New("GetArtists: stub is not promoted")
	}
	var aa = make([]Artist, len(t.STrack.Artist))
	for i, artist := range t.STrack.Artist {
		aa[i] = Artist{
			Id:     utils.ConvertTo62(artist.Gid),
			Name:   *artist.Name,
			Genres: artist.Genre,
			Stub:   artist,
		}
	}
	return aa, nil
}

// SanityCheck makes sure that all the tracks have an id associated with them because 90% of anything spotify related will silently fail. returns true if all is good
func SanityCheck(tracks []TrackStub) bool {
	for _, t := range tracks {
		if t.Id == "" {
			return false
		}
	}

	return true
}
