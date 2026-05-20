# TODO

1. [x] Theme UI + Theme UI Button
2. [x] Button for exiting (#1)
3. [NOT DOING] Default generate 'System' theme from terminal theme
4. [x] Dots next to file names on tabs to indicate if saving is necessary
5. [x] Ability to close file tabs
6. [x] toast <filename> opens toast in that directory and opens the file
7. [x] Support for word skipping with OPTION+ARROW and line skipping with CMD+ARRROW
8. [x] Create new files
9. [x] Syntax Highlighting shows for new/altered text
10. [x] I can't paste into files
11. [x] auto save option
12. [x] make git state in file tree clearer and look much nicer
13. [x] OPTION+BACKSPACE deletes words
14. [x] saving doesn't reset cursor position
15. [x] guard against opening binary files and crashing the text editor
16. [x] Markdown previewing in Toast (#2)
17. [ ] Add in a script to migrate zed themes to toast themes (#3)
18. [x] Add in a script to migrate vscode themes to toast themes
19. [x] Opening one tab, then another, and switching back to the first tab, the editor gets stuck on the second tab opened (#4)
20. [x] Scrolling is dependant on cursor position instead of smooth scrolling the editor (#5)
21. [ ] Add harper for markdown spelling (#6)
22. [ ] On small screens, the cursor gets lost as text goes beyond the editor bounds (#8)
23. [ ] Add a 'Save' button (#7)
24. [x] fix folders not showing git changes
25. [ ] Does this need a settings dialog? (#9)
26. [x] Resizable sidebar (#10)
27. [x] Mouse selection seems to be one char to the right? (#11)
28. [x] Markdown line wrapping (#12)
29. [x] `toast some-non-existant-file` creates a file and opens the editor there (#13)
30. [x] switching tabs causes unsaved text to be lost (#14)
31. [x] Ignored files/folders should be greyed out in the explorer (#15)
32. [x] 'System Theme' generation from terminal theme (#18)
33. [ ] More themes! Oh wow! (#17)
34. [ ] Markdown preview theme respect is grim and full of holes. Should nail it down more. (#19)
35. [ ] Emoji alters the cursor by the length of the emoji text in editor position making it seem off while actally rendering the emoji (#20)
36. [ ] Restore the Ghostty quit-flow screenshot test once the quit dialog shows the specific unsaved filenames and the capture can wait on that full state.
37. [ ] Restore the clean external-reload screenshot test once the app shows a visible reload notice so the golden proves the watcher-driven flow, not just the final buffer contents.
38. [ ] Restore the dirty external-change screenshot test once the app shows a visible "kept local edits" notice and the flow asserts the on-disk file remains the remote version after quitting.
39. [ ] Wire `Ctrl+N` / `Cmd+N` to a real new-file flow.
40. [ ] Project search results should jump to the selected match line/column instead of only opening the file.
41. [ ] Go-to-definition should jump to the returned line/column instead of only opening the target file.
42. [ ] Wire `editor.word_wrap` into the editor so wrap behavior is configurable instead of markdown-only.
43. [ ] Implement `editor.show_whitespace`.
44. [ ] Use configured `search.command` / `search.args` instead of hardcoding `rg --json`.
45. [ ] Make JavaScript LSP work out of the box with the default config.
