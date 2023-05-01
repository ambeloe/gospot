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
	"reflect"
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

// GetAudio returns the audio file in the desired format, starting at high bitrate then going lower if not available. Throws error if audio of type cannot be found.
func (s *Session) GetAudio(t TrackStub, format FormatType) (Audio, error) {
	var a Audio
	var fmts []int32
	var auds []*Spotify.AudioFile
	var aud *Spotify.AudioFile
	if t.STrack == nil {
		return Audio{}, errors.New("GetAudio: stub not promoted")
	}
	if auds = t.STrack.GetFile(); auds == nil {
		if DEBUG {
			fmt.Println(t.Id + ": ")
			util.PrintStruct(t)
		}
		if len(t.STrack.Alternative) > 0 {
			auds = t.STrack.Alternative[0].GetFile()
		} else {
			return a, errors.New("no audio found for track")
		}
	}

	//set desired format order
	switch format {
	case FormatMp3:
		//mp3: 320kbps, 256kbps, 160kbps, 96kbps
		fmts = []int32{4, 3, 5, 6}
	case FormatOgg:
		//ogg: 320kbps, 160kbps, 96kbps
		fmts = []int32{2, 1, 0}
	case FormatBestEffort:
		//highest bitrate first; ogg preferred in collisions
		fmts = []int32{2, 4, 3, 1, 5, 0, 6}
	default:
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

func (s *Session) GetPlaylist(id string, stubOnly bool) (Playlist, error) {
	var pl = Playlist{}

	_, err := s.ValidToken()
	if err != nil {
		return pl, err
	}
	ctx := context.Background()
	hc := spotifyauth.New().Client(ctx, s.Ls.OauthToken.ToOauthToken())
	client := spotify.NewClient(hc)
	client.AutoRetry = true

	apiPl, err := client.GetPlaylist(spotify.ID(id))
	if err != nil {
		return pl, errors.New("error getting playlist info: " + err.Error())
	}

	pl.Id = id
	pl.Name = apiPl.Name

	//get image
	pl.Thumb.File, err = util.GetFile(apiPl.Images[0].URL) //image struct won't have file id populated
	if err != nil {
		return pl, errors.New("error downloading playlist thumbnail image: " + err.Error())
	}

	pl.APIStub = apiPl

	//populate songs
	var t TrackStub
	pl.Len = apiPl.Tracks.Total
	pl.Songs = make([]TrackStub, 0, pl.Len)
	for {
		for _, apit := range apiPl.Tracks.Tracks {
			//fmt.Println(apit.Track.String())
			if apit.Track.ID == "" {
				//not ideal from a library, but this is the most elegant solution I came up with that doesn't resort to passing data through the error string
				fmt.Printf("Unplayable song found in playlist \"%s\": %s by %s (%d:%d)\n", pl.Name, apit.Track.Name, apit.Track.Artists[0].Name, apit.Track.Duration/(60*1000), (apit.Track.Duration/1000)%60)
			}
			if stubOnly {
				t, err = s.GetTrackStub(string(apit.Track.ID))
			} else {
				t, err = s.GetTrack(string(apit.Track.ID))

			}
			if err != nil {
				return pl, errors.New("error getting track during playlist population: " + err.Error())
			}

			if apit.Track.ID != "" {
				pl.Songs = append(pl.Songs, t)
			}

		}

		err = client.NextPage(&apiPl.Tracks)
		switch err {
		case nil:
		case spotify.ErrNoMorePages:
			return pl, nil
		default:
			return pl, errors.New("error getting tracks of playlist: " + err.Error())
		}
	}
}

//GetPlaylistMercury "same" as GetPlaylist but uses mercury api instead of the public api. Might be faster, but also can't get playlist images and likes to fail silently.
func (s *Session) GetPlaylistMercury(id string, stubOnly bool) (Playlist, error) {
	var pl = Playlist{}
	sc, err := s.Sess.Mercury().GetPlaylist(id)
	if err != nil {
		return pl, err
	}
	if reflect.DeepEqual(sc, Spotify.SelectedListContent{}) { //it really likes to not throw errors and return empty values
		return pl, errors.New("getting playlist failed for unknown reason")
	}
	var t TrackStub
	for _, e := range sc.Contents.Items {
		t = TrackStub{}
		g := util.URIStrip(e.GetUri())
		if stubOnly {
			t.Id = g
		} else {
			t, err = s.GetTrack(g)
			if err != nil {
				return pl, errors.New("GetPlaylist: error while getting tracks in playlist -- " + err.Error())
			}
		}

		pl.Songs = append(pl.Songs, t)
	}

	pl.Id = id
	//want to try to get "intended" length first to maybe allow some error detection
	if sc.Length != nil {
		pl.Len = int(*sc.Length)
	} else {
		pl.Len = len(pl.Songs)
	}
	if sc.Attributes != nil && sc.Attributes.Name != nil { //why must everything in this godforsaken library be a pointer
		pl.Name = *sc.Attributes.Name
	}
	//image not in mercury response???
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
		p, err = s.GetPlaylist(util.URIStrip(*e.Uri), true)
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
		wtf:
			t2, err2 := client.CurrentUsersTracksOpt(&spotify.Options{Limit: &lk, Offset: &loff})
			if err2 != nil {
				if err2.Error() == "Bad gateway." {
					err2 = nil
					goto wtf
				}
				return err2
			}
			//sometimes it will just return empty tracks
			if len(t2.Tracks) == 0 && tot%lk != 0 {
				goto wtf
			}
			for i, tt := range t2.Tracks {
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
	if !SanityCheck(trs) {
		return nil, errors.New("track list incomplete")
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

// ValidToken for internal use or if you don't want to maintain the token yourself; returns a valid token; uses the cached one if it is still valid, else gets a new one and updates the cache
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
	al.Id = id
	al.Name = *a.Name
	al.Artists = make([]Artist, 0)
	for _, aar := range a.Artist {
		ar, err := s.GetArtist(utils.ConvertTo62(aar.Gid))
		if err != nil {
			return al, errors.New("Error getting artist while getting album: " + err.Error())
		}
		al.Artists = append(al.Artists, ar)
	}
	al.Date = fmt.Sprintf("%04d-%02d-%02d", a.Date.Year, a.Date.Month, a.Date.Day)

	//get songs; librespot can't get album songs so the api is used to get them instead
	//yoinked from GetLikedSongs
	_, err = s.ValidToken()
	if err != nil {
		return al, err
	}
	ctx := context.Background()
	hc := spotifyauth.New().Client(ctx, s.Ls.OauthToken.ToOauthToken())
	client := spotify.NewClient(hc)
	client.AutoRetry = true

	fa, err := client.GetAlbum(spotify.ID(id))
	if err != nil {
		return al, errors.New("Error getting songs in album: " + err.Error())
	}
	for _, ttt := range fa.Tracks.Tracks { //feels like a footgun; better hope albums aren't long
		al.Songs = append(al.Songs, TrackStub{Id: string(ttt.ID)})
	}

	al.Stub = a
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
	*t = tt
	return nil
}
