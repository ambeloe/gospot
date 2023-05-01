module github.com/ambeloe/gospot

go 1.15

replace github.com/librespot-org/librespot-golang => github.com/ambeloe/librespot-golang v0.0.0-20230326014122-a3468800c742

//replace github.com/librespot-org/librespot-golang => /home/user/GolangProjects/librespot-golang

require (
	github.com/ambeloe/cliui v0.0.0-20210818225009-8f54c4b02123
	github.com/faiface/beep v1.1.0
	github.com/librespot-org/librespot-golang v0.0.0-20220325184705-31669e5a889f
	github.com/miekg/dns v1.1.43 // indirect
	github.com/zmb3/spotify v1.3.0
	github.com/zmb3/spotify/v2 v2.0.0
	golang.org/x/crypto v0.0.0-20210817164053-32db794688a5 // indirect
	golang.org/x/oauth2 v0.0.0-20210810183815-faf39c7919d5
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/sys v0.0.0-20210818153620-00dd8d7831e7 // indirect
	golang.org/x/term v0.0.0-20210615171337-6886f2dfbf5b
)
