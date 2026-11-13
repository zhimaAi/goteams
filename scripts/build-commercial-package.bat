@echo off
rem GoTeams commercial package build entry point; all logic lives in the PowerShell script.
rem Usage: build-commercial-package.bat [cn|dev2|dev3|dev6] [-Offline|-Both] [-MinUpgradeVersion vX.Y.Z]
rem   cn|dev2|dev3|dev6 selects the target build config (embedded INI + compose + signing key).
rem   -MinUpgradeVersion is the package manifest's lowest upgradable-from version (default v0.0.1,
rem   so any earlier commercial install may upgrade straight to the new package).
rem   -Offline builds the offline variant which embeds every compose image as
rem   payload/images/images.tar.gz so install/upgrade need no network access (docker required
rem   on the build machine; the archive is cached by image-list hash).
rem   -Both outputs the standard package and the offline package from one run: VERSION is occupied
rem   once and the frontend dist plus both Go binaries are compiled once, so the two .run files of
rem   the same release always carry the same version; each package still runs the whole assembly
rem   and verification chain on its own tree.
rem   With no argument the PowerShell script asks interactively; pressing Enter picks cn.
rem Both variants also carry payload/runtime/ (Docker Engine static tarball, Compose V2 plugin
rem binary and systemd units) so goteamsctl install can set up Docker on a host that has none;
rem those artifacts are downloaded from pinned URLs + SHA-256 on first build and cached under
rem build/commercial/vendor/docker, so later builds work offline.
rem The script holds an exclusive lock on build/commercial/VERSION and bumps the patch
rem version on every run; a failed build does not roll the version back.
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0build-commercial-package.ps1" %*
exit /b %ERRORLEVEL%
