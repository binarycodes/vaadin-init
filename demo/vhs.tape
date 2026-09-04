# The basic run, recorded: three answers changed, everything else accepted.
#
#     vhs demo/vhs.tape        # from the repository root
#
# Needs vhs, ttyd and ffmpeg (https://github.com/charmbracelet/vhs). The binary
# is built from this checkout at the start, off screen, and the project is written
# into a throwaway directory so the recording never touches the repository. The
# gif lands beside this file and is what the README opens with, so re-render it
# whenever the screen changes.

Output demo/demo.gif

Set Shell "bash"
Set FontSize 14
# 160 columns by 40 rows at this font size, which is comfortably above the
# ~150x36 the screen needs to tile; a smaller window records one section at a
# time instead. Adjust with the font: columns ≈ (Width - 2·Padding) / (0.6·FontSize).
Set Width 1400
Set Height 760
Set Padding 20
Set TypingSpeed 60ms
Set WindowBar Colorful
Set BorderRadius 10

# An identity for the first commit, so the Author section stays out of the
# recording and the summary ends on "first commit made".
Env GIT_AUTHOR_NAME "Ada Example"
Env GIT_AUTHOR_EMAIL "ada@example.com"
Env GIT_COMMITTER_NAME "Ada Example"
Env GIT_COMMITTER_EMAIL "ada@example.com"

Hide
Type `bin="$(mktemp -d)" && go build -o "$bin/vaadin-init" "$(git rev-parse --show-toplevel)" && export PATH="$bin:$PATH" && cd "$(mktemp -d)" && clear`
Enter
Sleep 2s
Show

Type "vaadin-init"
Sleep 500ms
Enter
Sleep 2s

# The fields open pre-filled, so each answer is cleared before it is typed.
Ctrl+U
Type "io.binarycodes"
Sleep 500ms
Enter
Sleep 500ms

Ctrl+U
Type "book-shelf"
Sleep 500ms
Enter
Sleep 500ms

Ctrl+U
Type "The Book Shelf"
Sleep 1.5s

# Enter accepts each remaining answer in turn — description, package, the three
# versions, the theme, the features, the directory — lands on Generate, and the
# tenth press is the one that generates.
Enter@600ms 10

# The project is written, committed, and reported on the same screen.
Sleep 5s

Type "verify"
Sleep 500ms
Enter
Sleep 5s

Type "quit"
Sleep 500ms
Enter
Sleep 1s