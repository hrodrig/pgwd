# pgwd terminal demo — run from repository root: vhs docs/demo.tape
# Requires: pgwd on PATH (e.g. make build && export PATH="$PWD:$PATH")
# Recorded output reflects current binary (version from VERSION file when built with make build).

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
