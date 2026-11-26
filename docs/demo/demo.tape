# Run docs/demo/setup.sh first, then: vhs docs/demo/demo.tape
Output docs/demo/demo.gif

Set Shell "bash"
Set FontSize 15
Set Width 1280
Set Height 680
Set Padding 16
Set Theme "Catppuccin Mocha"
Set TypingSpeed 90ms

Hide
Type "env -u TMUX tmux -L ccmdemo attach -t frontend" Enter
Sleep 1.5s
Show

Sleep 800ms
Ctrl+G
Sleep 3s
Type "j"
Sleep 600ms
Type "j"
Sleep 600ms
Type "k"
Sleep 1s

Type "/"
Sleep 400ms
Type "back"
Sleep 1.5s
Escape
Sleep 1s

Type "c"
Sleep 1.5s
Type "login"
Sleep 2s
Escape
Sleep 300ms
Escape
Sleep 1s

Type "1"
Sleep 2.5s
