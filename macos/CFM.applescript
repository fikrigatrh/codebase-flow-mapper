-- CFM droplet: drop a Go module folder on the app (or open it and pick one). It runs `cfm analyze` and
-- opens the report. Build with: osacompile -o dist/CFM.app macos/CFM.applescript
-- Reports go to ~/cfm-reports/<folder>.html, never into the analyzed repo.
-- ponytail: no --editor and no progress window; add them when someone asks.

on run
	set picked to choose folder with prompt "Pick the folder that holds the service's go.mod"
	analyze(picked)
end run

on open droppedItems
	repeat with f in droppedItems
		analyze(f)
	end repeat
end open

on analyze(f)
	set dir to POSIX path of f
	if dir ends with "/" and length of dir > 1 then set dir to text 1 thru -2 of dir
	set AppleScript's text item delimiters to "/"
	set folderName to last text item of dir
	set AppleScript's text item delimiters to ""
	set report to (POSIX path of (path to home folder)) & "cfm-reports/" & folderName & ".html"

	-- A login shell gives the user's PATH (Homebrew go); GOBIN or GOPATH/bin is where `go install` put cfm.
	set cmd to "export PATH=\"$(go env GOBIN):$(go env GOPATH)/bin:$PATH\"; mkdir -p ~/cfm-reports && cd " & quoted form of dir & " && cfm analyze . -o " & quoted form of report & " 2>&1; echo \"exit=$?\""
	set out to do shell script "/bin/zsh -lc " & quoted form of cmd

	set code to last paragraph of out
	if code is "exit=0" or code is "exit=1" then
		if code is "exit=1" then display notification "The target has load or type errors, so the graph is partial. Check the diagnostics." with title "CFM: " & folderName
		do shell script "open " & quoted form of report
	else
		display dialog "cfm could not analyze " & dir & return & return & out buttons {"OK"} default button "OK" with icon caution with title "CFM"
	end if
end analyze
