# pgwd terminal demo — from repository root, after installing pgwd on PATH:
#   make install && bash -c "vhs docs/demo.tape"
# Use bash -c (not bare `vhs` from zsh) so the recorder does not inherit zsh/Oh My Zsh prompts.
# Recorded output reflects the binary built with VERSION via make install / make build.

Output docs/demo.gif

Require pgwd

Set Shell "bash"
Set FontSize 14
Set Width 900
Set Height 500
Set Padding 20

# Clean prompt for recording
Enter
Sleep 300ms

# Help
Type "pgwd -h"
Enter
Sleep 2s

# Version
Type "pgwd --version"
Enter
Sleep 1s

# Validation: missing DB URL (shows required config)
Type "pgwd"
Enter
Sleep 2s

# Clean exit
Type "exit"
Enter
Sleep 500ms
