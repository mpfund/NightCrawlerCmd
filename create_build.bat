rm -rf build
mkdir build
xcopy .\websec.exe .\build\*
xcopy .\tags.json .\build\
xcopy .\resolv.conf .\build\
xcopy .\vectors.json .\build\
xcopy .\fuzzinginput.json .\build\
xcopy .\wordlists\* .\build\wordlists\ /s/i
