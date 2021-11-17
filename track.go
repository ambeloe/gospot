package gospot

import (
	"errors"
	"github.com/ambeloe/gospot/util"
	"github.com/librespot-org/librespot-golang/Spotify"
	"github.com/librespot-org/librespot-golang/librespot/utils"
)

type TrackStub struct {
	Id     string
	STrack *Spotify.Track `json:"-"`
}

type Track struct {
	Id string

	Title   string
	Artists []Artist
	Album   Album
	Number  int

	Art   Image `json:"-"`
	Sound Audio `json:"-"`

	Stub *Spotify.Track `json:"-"`
}

func (t TrackStub) GetImage() (Image, error) {
	var i Image
	var err error
	//try to find largest possible image
	for s := 3; s > -1; s-- {
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
