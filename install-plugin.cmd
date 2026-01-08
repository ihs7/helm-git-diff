@echo off
setlocal enabledelayedexpansion
cd /d "%HELM_PLUGIN_DIR%"
make build
