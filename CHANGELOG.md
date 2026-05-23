# Changelog

## [0.12.0](https://github.com/vitalvas/apt-transport-github/compare/v0.11.3...v0.12.0) (2026-05-23)


### Features

* add repository setup command ([0eb0338](https://github.com/vitalvas/apt-transport-github/commit/0eb03384a805d91348c66348f582ae959060976b))


### Bug Fixes

* change default versions from 3 to 1 ([e169b2a](https://github.com/vitalvas/apt-transport-github/commit/e169b2a1f0cfc4da98e4bba3269b52fa4d3a435a))
* handle apt transport edge cases ([c7fcda2](https://github.com/vitalvas/apt-transport-github/commit/c7fcda22fa793fb5cebc08647d0a8154cd73b2e4))

## [0.11.3](https://github.com/vitalvas/apt-transport-github/compare/v0.11.2...v0.11.3) (2026-05-08)


### Bug Fixes

* include system foreign architectures in Release metadata ([fcea611](https://github.com/vitalvas/apt-transport-github/commit/fcea611382393b5b8afe07d097773534a3d645a3))

## [0.11.2](https://github.com/vitalvas/apt-transport-github/compare/v0.11.1...v0.11.2) (2026-03-24)


### Bug Fixes

* send URI Start before URI Done for index file responses ([92a5b0c](https://github.com/vitalvas/apt-transport-github/commit/92a5b0cabcdca60ef8b5b44b0512660736d7b351))

## [0.11.1](https://github.com/vitalvas/apt-transport-github/compare/v0.11.0...v0.11.1) (2026-03-24)


### Bug Fixes

* add Last-Modified header to URI Done response ([053e8ef](https://github.com/vitalvas/apt-transport-github/commit/053e8ef5aea494b80959c838d46f0ab5f41063d0))

## [0.11.0](https://github.com/vitalvas/apt-transport-github/compare/v0.10.0...v0.11.0) (2026-03-24)


### Features

* support per-repo and per-owner token resolution ([2ea4776](https://github.com/vitalvas/apt-transport-github/commit/2ea47766a91b2bf037a2fcda00506509e38d3086))

## [0.10.0](https://github.com/vitalvas/apt-transport-github/compare/v0.9.0...v0.10.0) (2026-03-20)


### Features

* use GitHub API digest field as SHA256 fallback ([b00827d](https://github.com/vitalvas/apt-transport-github/commit/b00827dcc1579a787189e10924458e35bcb9e408))

## [0.9.0](https://github.com/vitalvas/apt-transport-github/compare/v0.8.0...v0.9.0) (2026-03-20)


### Features

* improve error reporting, SHA256 fallback, and embed APT sources file ([ee1634b](https://github.com/vitalvas/apt-transport-github/commit/ee1634bc5d9a8aee770675d072f9512cf6a793aa))


### Bug Fixes

* use GitHub API URL for authenticated asset downloads ([d53839b](https://github.com/vitalvas/apt-transport-github/commit/d53839bf951c268cdcf33690a99e01a192833178))

## [0.8.0](https://github.com/vitalvas/apt-transport-github/compare/v0.7.2...v0.8.0) (2026-03-20)


### Features

* rename project from apt-github to apt-transport-github ([77e4fb5](https://github.com/vitalvas/apt-transport-github/commit/77e4fb57b8bb485eb7214adbf932d52f4bbdd6a1))

## [0.7.2](https://github.com/vitalvas/apt-transport-github/compare/v0.7.1...v0.7.2) (2026-03-20)


### Bug Fixes

* change Release Origin from github to github.com ([a31f6d8](https://github.com/vitalvas/apt-transport-github/commit/a31f6d85eb930b68a71eec78138e97547a3fff85))

## [0.7.1](https://github.com/vitalvas/apt-transport-github/compare/v0.7.0...v0.7.1) (2026-03-20)


### Bug Fixes

* key control cache per deb filename to support multiple packages per release ([c84a5fe](https://github.com/vitalvas/apt-transport-github/commit/c84a5fe9922ff6acb409965b60e1091ce57691a7))

## [0.7.0](https://github.com/vitalvas/apt-transport-github/compare/v0.6.0...v0.7.0) (2026-03-20)


### Miscellaneous Chores

* release 0.7.0 ([5cac185](https://github.com/vitalvas/apt-transport-github/commit/5cac185165ee096db56b3bd02a5bdd63262bd5ef))

## [0.6.0](https://github.com/vitalvas/apt-transport-github/compare/v0.5.0...v0.6.0) (2026-03-20)


### Features

* cache observability, arch filtering, and stale version cleanup ([65c59a5](https://github.com/vitalvas/apt-transport-github/commit/65c59a5f294f54c5f3e065af3d1b8b8ae7ec7ab9))

## [0.5.0](https://github.com/vitalvas/apt-transport-github/compare/v0.4.0...v0.5.0) (2026-03-20)


### Features

* cache downloaded .deb packages to avoid re-downloading on install ([d605bde](https://github.com/vitalvas/apt-transport-github/commit/d605bde59dd1e4da9fcbd87d806fc0a07de76c01))

## [0.4.0](https://github.com/vitalvas/apt-transport-github/compare/v0.3.0...v0.4.0) (2026-03-20)


### Features

* add GitHub PAT support, dynamic version, and fix empty control fields ([3cc568f](https://github.com/vitalvas/apt-transport-github/commit/3cc568fdf0b9f6a05a7ccc4afd0b7e4f0db2ee3f))

## [0.3.0](https://github.com/vitalvas/apt-transport-github/compare/v0.2.0...v0.3.0) (2026-03-20)


### Features

* add multi-version support, disk cache, and clean command ([c0e5560](https://github.com/vitalvas/apt-transport-github/commit/c0e55603b3d9b9282e3621192aca20565beecf46))


### Bug Fixes

* use GitHub-supported admonition format for warning in README ([a1e7155](https://github.com/vitalvas/apt-transport-github/commit/a1e7155d002e251a9b8b2de495299cacb4857b6e))

## [0.2.0](https://github.com/vitalvas/apt-transport-github/compare/v0.1.0...v0.2.0) (2026-03-20)


### Features

* extract package dependencies from .deb control files ([3e0b0d7](https://github.com/vitalvas/apt-transport-github/commit/3e0b0d72d7ad45666332bd589eabf1d1f397b311))

## [0.1.0](https://github.com/vitalvas/apt-transport-github/compare/v0.0.1...v0.1.0) (2026-03-20)


### Features

* initial implementation of APT transport method for GitHub releases ([8a39197](https://github.com/vitalvas/apt-transport-github/commit/8a39197a857e55c41fb761037f88eabdcf3e1dbe))
