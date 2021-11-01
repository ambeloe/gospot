package gospot

import (
	"errors"
	"fmt"
	"github.com/ambeloe/gospot/util"
	"github.com/librespot-org/librespot-golang/Spotify"
	"github.com/librespot-org/librespot-golang/librespot/core"
	"github.com/librespot-org/librespot-golang/librespot/metadata"
	"github.com/librespot-org/librespot-golang/librespot/utils"
	"io"
	"reflect"
	"time"
)

type Session struct {
	Ls   *LocalStore
	Sess *core.Session
}

func (s *Session) GetTrack(trackId string) (TrackStub, error) {
	var err error
	var t TrackStub
	t.Id = trackId
	t.STrack, err = s.Sess.Mercury().GetTrack(utils.Base62ToHex(trackId))
	if t.STrack.Gid == nil {
		return t, errors.New("Song not found.")
	}
	return t, err
}

// GetAudio returns the audio file in the desired format(mp3 or ogg), starting at high bitrate then going lower if not available. Throws error if audio of type cannot be found.
func (s *Session) GetAudio(t TrackStub, format FormatType) (Audio, error) {
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
		time.Sleep(10e6)
	}
	a.File = &ogg
	return a, err
}

//todo: see if long playlists are broken
//GetPlaylist worked earlier; broken again
func (s *Session) GetPlaylist(id string) (Playlist, error) {
	var pl = Playlist{}
	sc, err := s.Sess.Mercury().GetPlaylist(utils.Base62ToHex(id)) //
	if err != nil {
		return pl, err
	}
	if reflect.DeepEqual(sc, Spotify.SelectedListContent{}) { //it really likes to not throw errors and return empty values
		return pl, errors.New("getting playlist failed for unknown reason")
	}
	var t TrackStub
	for _, e := range sc.Contents.Items {
		g := util.URIStrip(e.GetUri())
		t, err = s.GetTrack(g)
		if err != nil {
			return pl, errors.New("GetPlaylist: error while getting tracks in playlist -- " + err.Error())
		}
		pl.Songs = append(pl.Songs, t)
	}
	return pl, nil
}

//GetRootPlaylist also broken again for some reason
func (s *Session) GetRootPlaylist() ([]Playlist, error) {
	var pls = make([]Playlist, 0)
	sc, err := s.Sess.Mercury().GetRootPlaylist(s.Sess.Username()) //not clue why it even has an argument if you can only get your own
	//todo: fails silently; add checks for all nil and shit
	if err != nil {
		return pls, errors.New("error getting root playlist")
	}
	var p Playlist
	for _, e := range sc.Contents.GetItems() {
		p, err = s.GetPlaylist(util.URIStrip(e.GetUri()))
		if err != nil {
			return pls, errors.New("error while getting playlist in root: " + err.Error())
		}
		pls = append(pls, p)
	}
	return pls, nil
}

func (s *Session) GetLikedSongs() ([]TrackStub, error) {
	return nil, nil
}

//GetOauthToken gets an access token with all scopes if scope is empty or with the specified scopes
func (s *Session) GetOauthToken(scope string) (*metadata.Token, error) {
	if scope == "" {
		return s.Sess.Mercury().GetToken("2c1ea588dfbc4a989e2426f8385297c3", "ugc-image-upload,playlist-modify-private,playlist-read-private,playlist-modify-public,playlist-read-collaborative,user-read-private,user-read-email,user-read-playback-state,user-modify-playback-state,user-read-currently-playing,user-library-modify,user-library-read,user-read-playback-position,user-read-recently-played,user-top-read,app-remote-control,streaming,user-follow-modify,user-follow-read") //they got it from somewhere lmao https://github.com/Spotifyd/spotifyd/issues/507
	} else {
		return s.Sess.Mercury().GetToken("2c1ea588dfbc4a989e2426f8385297c3", scope)
	}
}

// ValidToken for internal use or if you dont want to maintain the token yourself; returns a valid token; uses the cached one if it is still valid, else gets a new one and updates the cache
func (s *Session) ValidToken() (string, error) {
	var err error
	if s.Ls.OauthToken.Token != nil && !s.Ls.OauthToken.IssueTime.IsZero() {
		if time.Now().Add(time.Minute).After(s.Ls.OauthToken.IssueTime.Add(time.Duration(s.Ls.OauthToken.Token.ExpiresIn) * time.Second)) { //see if the token is about to expire in a minute
			s.Ls.OauthToken.Token, err = s.GetOauthToken("")
			if err != nil {
				return "", err
			}
			s.Ls.OauthToken.IssueTime = time.Now()
		}
	} else { //too tired to care
		s.Ls.OauthToken.Token, err = s.GetOauthToken("")
		if err != nil {
			return "", err
		}
		s.Ls.OauthToken.IssueTime = time.Now()
	}
	return s.Ls.OauthToken.Token.AccessToken, nil
}
