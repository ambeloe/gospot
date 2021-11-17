module github.com/ambeloe/gospot

go 1.15

replace github.com/librespot-org/librespot-golang => github.com/ambeloe/librespot-golang v0.0.0-20200423180623-b19a2f10c856
//replace github.com/librespot-org/librespot-golang => /home/user/go/src/github.com/ambeloe/librespot-golang

require (
	github.com/ambeloe/cliui v0.0.0-20210818225009-8f54c4b02123
	github.com/faiface/beep v1.1.0
	github.com/librespot-org/librespot-golang v0.0.0-20200423180623-b19a2f10c856
	github.com/miekg/dns v1.1.43 // indirect
	github.com/zmb3/spotify v1.3.0
	github.com/zmb3/spotify/v2 v2.0.0
	golang.org/x/crypto v0.0.0-20210817164053-32db794688a5 // indirect
	golang.org/x/oauth2 v0.0.0-20210810183815-faf39c7919d5
	golang.org/x/sys v0.0.0-20210818153620-00dd8d7831e7 // indirect
	golang.org/x/term v0.0.0-20210615171337-6886f2dfbf5b
)
