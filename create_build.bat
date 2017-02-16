rem rm -rf build
rem rmdir /s /q build
mkdir build
xcopy ".\ncrawler.exe" ".\build\*" /y
xcopy .\config\* .\build\config\ /s/y
xcopy .\wordlist\* .\build\wordlist\ /s/y
xcopy .\template\* .\build\template\ /s/y
