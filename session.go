package gospot

import (
	"context"
	"errors"
	"fmt"
	"github.com/ambeloe/gospot/util"
	"github.com/librespot-org/librespot-golang/Spotify"
	"github.com/librespot-org/librespot-golang/librespot/core"
	"github.com/librespot-org/librespot-golang/librespot/metadata"
	"github.com/librespot-org/librespot-golang/librespot/utils"
	"github.com/zmb3/spotify"
	"github.com/zmb3/spotify/v2/auth"
	"golang.org/x/sync/errgroup"
	"io"
	"sync"
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
		return t, errors.New("GetTrack: song not found")
	}
	return t, err
}

func (s *Session) GetTrackFull(stub TrackStub) (Track, error) {
	var t Track
	err := s.PromoteStub(&stub)
	if err != nil {
		return t, err
	}
	t.Id = stub.Id
	t.Title = *stub.STrack.Name
	a, err := stub.GetArtists()
	if err != nil {
		return t, err
	}
	t.Artists = a
	al, err := s.GetAlbum(utils.ConvertTo62(stub.STrack.Album.Gid))
	if err != nil {
		return t, err
	}
	t.Album = al
	t.Number = int(*stub.STrack.Number)
	im, err := stub.GetImage()
	if err != nil {
		return t, err
	}
	t.Art = im
	son, err := s.GetAudio(stub, FormatOgg)
	if err != nil {
		return t, err
	}
	t.Sound = son
	return t, nil
}

func (s *Session) GetTrackStub(trackId string) (TrackStub, error) {
	return TrackStub{Id: trackId, STrack: nil}, nil
}

// GetAudio returns the audio file in the desired format(mp3 or ogg), starting at high bitrate then going lower if not available. Throws error if audio of type cannot be found.
func (s *Session) GetAudio(t TrackStub, format FormatType) (Audio, error) {
	var a Audio
	var fmts []int32
	var auds []*Spotify.AudioFile
	var aud *Spotify.AudioFile
	if t.STrack == nil {
		return Audio{}, errors.New("GetAudio: stub not promoted")
	}
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
			if int32(*tr.Format) == i {
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
	var ogg = make([]byte, decaud.Size())
	var cnk = make([]byte, 128*1024)

	pos := 0
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
func (s *Session) GetPlaylist(id string, stubOnly bool) (Playlist, error) {
	var pl = Playlist{}
	sc, err := s.Sess.Mercury().GetPlaylist(id)
	//sc, err := s.Sess.Mercury().GetPlaylist(utils.Base62ToHex(id))
	if err != nil {
		return pl, err
	}
	if sc.Attributes == nil || sc.Attributes.Name == nil || sc.Length == nil {
		return pl, errors.New("GetPlaylist: failed for unknown reason")
	}
	pl.Id = id
	pl.Name = *sc.Attributes.Name
	pl.Len = int(*sc.Length)

	pl.Thumb.Id = sc.Attributes.Picture
	pl.Thumb.File, err = util.GetImageFile(pl.Thumb.Id)
	if err != nil {
		return pl, errors.New("GetPlaylist: error getting playlist image")
	}
	var t TrackStub
	if pl.Len != 0 && (sc.Contents == nil || sc.Contents.Items == nil) {
		return pl, errors.New("GetPlaylist: reported length isn't zero, but contents are nil")
	}
	if sc.Contents != nil && sc.Contents.Items != nil {
		if pl.Len != len(sc.Contents.Items) && !*sc.Contents.Truncated {
			return pl, errors.New("GetPlaylist: reported length of contents and real length mismatch")
		}
		for _, e := range sc.Contents.Items {
			g := util.URIStrip(e.GetUri())
			if stubOnly {
				t, err = s.GetTrackStub(g)
			} else {
				t, err = s.GetTrack(g)
			}
			if err != nil {
				return pl, errors.New("GetPlaylist: error while getting tracks in playlist -- " + err.Error())
			}
			pl.Songs = append(pl.Songs, t)
		}
	}
	return pl, nil
}

func (s *Session) GetRootPlaylist() ([]Playlist, error) {
	var pls = make([]Playlist, 0)
	sc, err := s.Sess.Mercury().GetRootPlaylist(s.Sess.Username()) //not clue why it even has an argument if you can only get your own
	//todo: fails silently; add checks for all nil and shit
	if err != nil {
		return pls, errors.New("error getting root playlist")
	}
	var p Playlist
	for _, e := range sc.Contents.Items {
		p, err = s.GetPlaylist(util.URIStrip(e.GetUri()), true)
		if err != nil {
			return pls, errors.New("error while getting playlist in root: " + err.Error())
		}
		pls = append(pls, p)
	}
	return pls, nil
}

//GetLikedSongs may take forever if you have a ton of songs since it only gets them in chunks of 50
func (s *Session) GetLikedSongs() ([]TrackStub, error) {
	var trMut = sync.Mutex{}
	var trs []TrackStub
	_, err := s.ValidToken()
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	hc := spotifyauth.New().Client(ctx, s.Ls.OauthToken.ToOauthToken())
	client := spotify.NewClient(hc)
	client.AutoRetry = true
	k, off, tot := 50, 0, 0

	offmut := sync.Mutex{}

	t, err := client.CurrentUsersTracks()
	if err != nil {
		return trs, nil
	}
	tot = t.Total
	trs = make([]TrackStub, tot)

	var erg = new(errgroup.Group)
	for {
		offmut.Lock()
		erg.Go(func() error {
			lk := k
			loff := off
			offmut.Unlock()
			t, err := client.CurrentUsersTracksOpt(&spotify.Options{Limit: &lk, Offset: &loff})
			if err != nil {
				return err
			}
			for i, tt := range t.Tracks {
				trMut.Lock()
				trs[loff+i] = TrackStub{Id: tt.FullTrack.ID.String()}
				trMut.Unlock()
				if loff+i >= tot-1 {
					break
				}
			}
			return nil
		})
		offmut.Lock()
		off += k
		offmut.Unlock()
		if off >= tot {
			break
		}
	}
	//fmt.Println("threads spawned, waiting...")
	if err = erg.Wait(); err != nil {
		return trs, err
	}
	return trs, nil
}

//GetOauthToken gets an access token with all scopes if scope is empty or with the specified scopes
func (s *Session) GetOauthToken(scope string) (*metadata.Token, error) {
	if scope == "" {
		//they got it from somewhere lmao https://github.com/Spotifyd/spotifyd/issues/507
		return s.Sess.Mercury().GetToken("2c1ea588dfbc4a989e2426f8385297c3", "ugc-image-upload,playlist-modify-private,playlist-read-private,playlist-modify-public,playlist-read-collaborative,user-read-private,user-read-email,user-read-playback-state,user-modify-playback-state,user-read-currently-playing,user-library-modify,user-library-read,user-read-playback-position,user-read-recently-played,user-top-read,app-remote-control,streaming,user-follow-modify,user-follow-read")
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

func (s *Session) GetArtist(id string) (Artist, error) {
	var ar Artist
	a, err := s.Sess.Mercury().GetArtist(utils.Base62ToHex(id))
	if err != nil {
		return ar, errors.New("GetArtist: unknown error, " + err.Error())
	}
	if a.Name == nil {
		return ar, errors.New("GetArtist: name was empty")
	}
	ar.Id = id
	ar.Name = *a.Name
	ar.Genres = a.Genre
	ar.Stub = a
	return ar, nil
}

func (s *Session) GetAlbum(id string) (Album, error) {
	var al Album
	a, err := s.Sess.Mercury().GetAlbum(utils.Base62ToHex(id))
	if err != nil {
		return al, errors.New("GetAlbum: unknown error, " + err.Error())
	}
	if a.Gid == nil || a.Name == nil {
		return al, errors.New("GetAlbum: required fields nil")
	}
	return al, nil
}

func (s *Session) PromoteStub(t *TrackStub) error {
	if t.STrack != nil {
		return nil
	}
	tt, err := s.GetTrack(t.Id)
	if err != nil {
		return errors.New("PromoteStub: error promoting stub")
	}
	t = &tt
	return nil
}
