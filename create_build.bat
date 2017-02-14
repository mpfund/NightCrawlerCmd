#rm -rf build
#rmdir /s /q build
mkdir build
xcopy .\websec.exe .\build\* /s/i
xcopy .\config\* .\build\config\ /s/i
xcopy .\wordlist\* .\build\wordlist\ /s/i
xcopy .\template\* .\build\template\ /s/i
