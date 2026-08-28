# Changelog

## [0.4.4](https://github.com/abn/coprctl/compare/v0.4.3...v0.4.4) (2026-08-28)


### Bug Fixes

* **packaging:** use a release-please version block so rpmbuild can parse the spec ([#10](https://github.com/abn/coprctl/issues/10)) ([77c9ae7](https://github.com/abn/coprctl/commit/77c9ae7e5fb4b168ecbb9a35117024e0724072a5))


### Miscellaneous Chores

* trigger 0.4.4 release ([65e9166](https://github.com/abn/coprctl/commit/65e9166ce461f6a13b101de924f1edf8c84a2289))

## [0.4.3](https://github.com/abn/coprctl/compare/v0.4.2...v0.4.3) (2026-08-28)


### Miscellaneous Chores

* trigger 0.4.3 release ([1ca9399](https://github.com/abn/coprctl/commit/1ca9399e764dfdc49d858a232305dea49192bfdd))

## [0.4.2](https://github.com/abn/coprctl/compare/v0.4.1...v0.4.2) (2026-08-28)


### Miscellaneous Chores

* release 0.4.2 ([43a2a3a](https://github.com/abn/coprctl/commit/43a2a3a3e67b829798fd9501b27e657fa36d81d3))

## [0.4.1](https://github.com/abn/coprctl/compare/v0.4.0...v0.4.1) (2026-08-28)


### Bug Fixes

* scope the webhook to a package so bare release tags match ([7703c33](https://github.com/abn/coprctl/commit/7703c337ba217500dee741671d459bcbc5175e1f))

## [0.4.0](https://github.com/abn/coprctl/compare/coprctl-v0.3.0...coprctl-v0.4.0) (2026-08-28)


### Features

* add --global flag to skill install ([d1c28bc](https://github.com/abn/coprctl/commit/d1c28bc51009f174bc9184256ac4ac159a42cc08))
* add ANSI color to human table output on a TTY ([ab71dc8](https://github.com/abn/coprctl/commit/ab71dc83416391ea50feb48ba578694857347e80))
* add build srpm and chain local SRPM into submit ([30dc389](https://github.com/abn/coprctl/commit/30dc389fb328ca89068dffcb7e85b6005318c8c3))
* add build submit --watch to wait for a submitted build ([99b9801](https://github.com/abn/coprctl/commit/99b98012f39a8e9c90c869bf2870310e187e739c))
* add coprctl landing page ([674f39b](https://github.com/abn/coprctl/commit/674f39b78c01624c1a3d1bc5798b622f82e6679f))
* add intent-based build backends with container fallback ([6ae9a7b](https://github.com/abn/coprctl/commit/6ae9a7b81b0135b153764296f90f5157ec7b8919))
* add secret handlers, edit coverage, build watch, and TUI ([666c50f](https://github.com/abn/coprctl/commit/666c50f3575d4fbdd7ec73407e4b2355386ce9d1))
* **agent:** add MCP server, generated skill, and drift gate ([8ac2a7c](https://github.com/abn/coprctl/commit/8ac2a7cc9931e39fbb2fd24db57429f2693ad717))
* **auth:** add auth login browser flow ([34c630d](https://github.com/abn/coprctl/commit/34c630dc0a64ca145aee7819d5ff2d31ce251a0e))
* **auth:** add token status, rotation, and config import ([62cbc6d](https://github.com/abn/coprctl/commit/62cbc6d46bf8f1eea65fcc52b6918b83f7ab4848))
* **cli:** add completions, doctor, and release packaging ([92f0f51](https://github.com/abn/coprctl/commit/92f0f5126e6d05b8df6b900c7e933c1d85d64193))
* **cli:** add CRUD parity and build lifecycle ([83c00eb](https://github.com/abn/coprctl/commit/83c00eb9bb1f62f9887fae237975874fa68e53eb))
* **cli:** add M0 skeleton with registry, parser, client ([422429d](https://github.com/abn/coprctl/commit/422429db083fac6be596ed45ac3862929aa04782))
* complete chroot names and project refs from cache and API ([a03c0b1](https://github.com/abn/coprctl/commit/a03c0b1e3370a037dc37b9cee9b51e3186b1ef8b))
* **config:** recognize openEuler and self-hosted instances ([7c5e54c](https://github.com/abn/coprctl/commit/7c5e54c56fb31ae262a18d0fd18477d3a571b526))
* **debug:** add build debugging, skill bundle, and config migration ([74facc3](https://github.com/abn/coprctl/commit/74facc3ba8d4fb5b24a9d770378f1aa320a4f50e))
* derive homepage and issues contact from a linked GitHub repo ([80a070c](https://github.com/abn/coprctl/commit/80a070cde4e402136a7cccb01f0f881b4aa2fc36))
* enable chroot retirement via project chroot and apply --prune ([8e8a126](https://github.com/abn/coprctl/commit/8e8a126d4a825546e0f61a77ddaa145a614d98f8))
* enable webhook auto-rebuild implicitly when adding an integration ([ee40113](https://github.com/abn/coprctl/commit/ee40113baca6c8e3b37b699ee831e5ec05370071))
* expose network access setting on project create and edit ([38f7467](https://github.com/abn/coprctl/commit/38f7467a15100f3a70b561dfab2d0a1e39da0614))
* **init:** add detect, init, and sync commands ([ea846a3](https://github.com/abn/coprctl/commit/ea846a3bf15ea5c58bafb3cae12cf75310ccc815))
* **integration:** default webhook to tag-only triggers ([e0134f3](https://github.com/abn/coprctl/commit/e0134f37eabe6390b2a253d9b1c053c87f6a1b21))
* live auth validity check in auth status ([f866712](https://github.com/abn/coprctl/commit/f866712fcc9e9f9d226aaba9152ca410a22f58e0))
* **logs:** add event bus, log tailer, monitor, and status ([93118c0](https://github.com/abn/coprctl/commit/93118c0670947390e853cedf66b06fee81051ba8))
* **manifest:** add declarative state and GitHub integration ([201f325](https://github.com/abn/coprctl/commit/201f325bf9d5f86095a2abbf679c9a5709b1a65c))
* **preflight:** add try with container runtime and fidelity report ([80d55f7](https://github.com/abn/coprctl/commit/80d55f7d19c7879832ba3d43dfc838c8aa2452c0))
* rebuild only failed chroots with --only-failed ([d7c78c0](https://github.com/abn/coprctl/commit/d7c78c098b160f2afaa2a79a2110f3eace418861))
* skill install defaults to .agents/skills and installs all skills ([8bc7757](https://github.com/abn/coprctl/commit/8bc7757c82eaa22bd9305a3f99570a3e9f9279d6))
* support installation instructions on project create and edit ([1379e08](https://github.com/abn/coprctl/commit/1379e08d8cd355b3f3b8b68884574297e8f314fb))
* surface chroot EOL state in chroot list ([250a0de](https://github.com/abn/coprctl/commit/250a0decf2c5bba33d8bfc4a6703b9279ef06235))
* surface enable_net in project get ([bc60f67](https://github.com/abn/coprctl/commit/bc60f6735aa63ef35a698bcffb7955264aa1c96f))
* **ui:** add project view, doctor exit codes, and buildable spec ([e4a25cc](https://github.com/abn/coprctl/commit/e4a25cc6292595016f7f1735fb2ea98253116bd2))


### Bug Fixes

* **auth:** write rotated credentials back to the legacy config ([2d00284](https://github.com/abn/coprctl/commit/2d002843bfb002cd6dde02ec3cc5ae0bb75933b6))
* **cli:** allow anonymous reads without configuration ([2d7da52](https://github.com/abn/coprctl/commit/2d7da529a73ff2907589dffb0cddd170a98b2d94))
* cross-platform tests and update project domain ([3d26470](https://github.com/abn/coprctl/commit/3d26470439d281d2da88960f9128fd92aa706873))
* default project list to the current user ([1b72564](https://github.com/abn/coprctl/commit/1b725645231f3fd68e9790e7966b2a85c0a74814))
* **integration:** match hook by webhook destination and poll ping ([cc52ad6](https://github.com/abn/coprctl/commit/cc52ad640898f9cea549543903bab20b894bd851))
* make local SRPM build and upload work end to end ([cfda48c](https://github.com/abn/coprctl/commit/cfda48c81d198ecb27b93b6c3a0325fcaaca3957))
* make stdin read and container mounts portable ([daff57d](https://github.com/abn/coprctl/commit/daff57d293a9abe498da4d49864cc4dc803ac207))
* reconcile project chroots via the verified project edit endpoint ([f1ac034](https://github.com/abn/coprctl/commit/f1ac0348863b6b2cd2be20f90744fbc8dd4b6645))
* support side-repo dirs and release-please specs in local builds ([19b06ba](https://github.com/abn/coprctl/commit/19b06bac9b6f1a58255c3b2986bb794d88026ae8))


### Documentation

* add ADRs for implemented decisions and drop repeated titles ([c858d28](https://github.com/abn/coprctl/commit/c858d28a26b9a3a9427061041a7a7065a18f9ff7))
* add project README ([9494d32](https://github.com/abn/coprctl/commit/9494d32df9a70b988e3474bc53a6cbd8b56a38bb))
* add release process and sync spec version with release-please ([27f6ce2](https://github.com/abn/coprctl/commit/27f6ce25029fa8b8581152df2d3ea9dcfff494d5))
* complete the wiki to reflect status quo ([e2a0e8e](https://github.com/abn/coprctl/commit/e2a0e8e34cb5baaa01ed674a8afca827ffead1aa))
* correct config library in ADR 0001 ([c00e52a](https://github.com/abn/coprctl/commit/c00e52a547991ca677db13d0af2c2624b2acc88f))
* fold install instructions into the quick start ([b34c6c2](https://github.com/abn/coprctl/commit/b34c6c256dc1ee4385a992484b198bc0346ded92))
* point to upstream Copr source as ground truth ([ef9edf5](https://github.com/abn/coprctl/commit/ef9edf54c4e0d21d29be7559b840d10d66519817))
* record chroot lifecycle and retire workflows ([d088658](https://github.com/abn/coprctl/commit/d0886581e3e6c657e2eca615fb763e3c369c6f5a))
* record completion of all project waves ([4e6b643](https://github.com/abn/coprctl/commit/4e6b643cc422bf60dadd309a14db0e421c85b9a6))
* record cross-platform release decision ([f4a4695](https://github.com/abn/coprctl/commit/f4a4695a7e664bcc58bb72577d292f92e7721549))
* scope the wiki log to knowledge-base evolution ([55f4590](https://github.com/abn/coprctl/commit/55f45900381aef723fcb3a253cf8707e00cbc065))
* standardize project and binary name on coprctl ([af7d0ca](https://github.com/abn/coprctl/commit/af7d0ca23f5e1801db88d2197b87b95de1922183))


### Build & Packaging

* **deps:** bump the github-actions group across 1 directory with 3 updates ([#1](https://github.com/abn/coprctl/issues/1)) ([5598c50](https://github.com/abn/coprctl/commit/5598c50fd14c60b10a9ed916a8775223697e4718))
* drop rpm and deb packages from releases ([dafc4ec](https://github.com/abn/coprctl/commit/dafc4ec3ab32e271338bedb221f95c9c5dd14a27))
* name release tags so Copr can match the package ([9093dd1](https://github.com/abn/coprctl/commit/9093dd1597c8de583b8a7dc8bc3f995597963653))
* set project homepage to coprctl.abn.is ([ebdf817](https://github.com/abn/coprctl/commit/ebdf817bf8831243105259bb477720dc90615f93))


### Continuous Integration

* add GitHub Actions, release-please, and dependabot ([efc644b](https://github.com/abn/coprctl/commit/efc644bbd4337b85410fb57918eb2b7ba6f3fac7))
* verify builds and tests across release platforms ([aa0606c](https://github.com/abn/coprctl/commit/aa0606cb69c51c589ebf137151b8c1002be3b550))

## [0.3.0](https://github.com/abn/coprctl/compare/v0.2.0...v0.3.0) (2026-08-28)


### Features

* derive homepage and issues contact from a linked GitHub repo ([80a070c](https://github.com/abn/coprctl/commit/80a070cde4e402136a7cccb01f0f881b4aa2fc36))
* enable webhook auto-rebuild implicitly when adding an integration ([ee40113](https://github.com/abn/coprctl/commit/ee40113baca6c8e3b37b699ee831e5ec05370071))
* support installation instructions on project create and edit ([1379e08](https://github.com/abn/coprctl/commit/1379e08d8cd355b3f3b8b68884574297e8f314fb))


### Documentation

* fold install instructions into the quick start ([b34c6c2](https://github.com/abn/coprctl/commit/b34c6c256dc1ee4385a992484b198bc0346ded92))


### Build & Packaging

* **deps:** bump the github-actions group across 1 directory with 3 updates ([#1](https://github.com/abn/coprctl/issues/1)) ([5598c50](https://github.com/abn/coprctl/commit/5598c50fd14c60b10a9ed916a8775223697e4718))
* drop rpm and deb packages from releases ([dafc4ec](https://github.com/abn/coprctl/commit/dafc4ec3ab32e271338bedb221f95c9c5dd14a27))

## [0.2.0](https://github.com/abn/coprctl/compare/v0.1.0...v0.2.0) (2026-08-28)


### Features

* add --global flag to skill install ([d1c28bc](https://github.com/abn/coprctl/commit/d1c28bc51009f174bc9184256ac4ac159a42cc08))
* add ANSI color to human table output on a TTY ([ab71dc8](https://github.com/abn/coprctl/commit/ab71dc83416391ea50feb48ba578694857347e80))
* add build srpm and chain local SRPM into submit ([30dc389](https://github.com/abn/coprctl/commit/30dc389fb328ca89068dffcb7e85b6005318c8c3))
* add build submit --watch to wait for a submitted build ([99b9801](https://github.com/abn/coprctl/commit/99b98012f39a8e9c90c869bf2870310e187e739c))
* add coprctl landing page ([674f39b](https://github.com/abn/coprctl/commit/674f39b78c01624c1a3d1bc5798b622f82e6679f))
* add intent-based build backends with container fallback ([6ae9a7b](https://github.com/abn/coprctl/commit/6ae9a7b81b0135b153764296f90f5157ec7b8919))
* add secret handlers, edit coverage, build watch, and TUI ([666c50f](https://github.com/abn/coprctl/commit/666c50f3575d4fbdd7ec73407e4b2355386ce9d1))
* **agent:** add MCP server, generated skill, and drift gate ([8ac2a7c](https://github.com/abn/coprctl/commit/8ac2a7cc9931e39fbb2fd24db57429f2693ad717))
* **auth:** add auth login browser flow ([34c630d](https://github.com/abn/coprctl/commit/34c630dc0a64ca145aee7819d5ff2d31ce251a0e))
* **auth:** add token status, rotation, and config import ([62cbc6d](https://github.com/abn/coprctl/commit/62cbc6d46bf8f1eea65fcc52b6918b83f7ab4848))
* **cli:** add completions, doctor, and release packaging ([92f0f51](https://github.com/abn/coprctl/commit/92f0f5126e6d05b8df6b900c7e933c1d85d64193))
* **cli:** add CRUD parity and build lifecycle ([83c00eb](https://github.com/abn/coprctl/commit/83c00eb9bb1f62f9887fae237975874fa68e53eb))
* **cli:** add M0 skeleton with registry, parser, client ([422429d](https://github.com/abn/coprctl/commit/422429db083fac6be596ed45ac3862929aa04782))
* complete chroot names and project refs from cache and API ([a03c0b1](https://github.com/abn/coprctl/commit/a03c0b1e3370a037dc37b9cee9b51e3186b1ef8b))
* **config:** recognize openEuler and self-hosted instances ([7c5e54c](https://github.com/abn/coprctl/commit/7c5e54c56fb31ae262a18d0fd18477d3a571b526))
* **debug:** add build debugging, skill bundle, and config migration ([74facc3](https://github.com/abn/coprctl/commit/74facc3ba8d4fb5b24a9d770378f1aa320a4f50e))
* enable chroot retirement via project chroot and apply --prune ([8e8a126](https://github.com/abn/coprctl/commit/8e8a126d4a825546e0f61a77ddaa145a614d98f8))
* **init:** add detect, init, and sync commands ([ea846a3](https://github.com/abn/coprctl/commit/ea846a3bf15ea5c58bafb3cae12cf75310ccc815))
* **integration:** default webhook to tag-only triggers ([e0134f3](https://github.com/abn/coprctl/commit/e0134f37eabe6390b2a253d9b1c053c87f6a1b21))
* live auth validity check in auth status ([f866712](https://github.com/abn/coprctl/commit/f866712fcc9e9f9d226aaba9152ca410a22f58e0))
* **logs:** add event bus, log tailer, monitor, and status ([93118c0](https://github.com/abn/coprctl/commit/93118c0670947390e853cedf66b06fee81051ba8))
* **manifest:** add declarative state and GitHub integration ([201f325](https://github.com/abn/coprctl/commit/201f325bf9d5f86095a2abbf679c9a5709b1a65c))
* **preflight:** add try with container runtime and fidelity report ([80d55f7](https://github.com/abn/coprctl/commit/80d55f7d19c7879832ba3d43dfc838c8aa2452c0))
* rebuild only failed chroots with --only-failed ([d7c78c0](https://github.com/abn/coprctl/commit/d7c78c098b160f2afaa2a79a2110f3eace418861))
* skill install defaults to .agents/skills and installs all skills ([8bc7757](https://github.com/abn/coprctl/commit/8bc7757c82eaa22bd9305a3f99570a3e9f9279d6))
* surface chroot EOL state in chroot list ([250a0de](https://github.com/abn/coprctl/commit/250a0decf2c5bba33d8bfc4a6703b9279ef06235))
* **ui:** add project view, doctor exit codes, and buildable spec ([e4a25cc](https://github.com/abn/coprctl/commit/e4a25cc6292595016f7f1735fb2ea98253116bd2))


### Bug Fixes

* **auth:** write rotated credentials back to the legacy config ([2d00284](https://github.com/abn/coprctl/commit/2d002843bfb002cd6dde02ec3cc5ae0bb75933b6))
* **cli:** allow anonymous reads without configuration ([2d7da52](https://github.com/abn/coprctl/commit/2d7da529a73ff2907589dffb0cddd170a98b2d94))
* cross-platform tests and update project domain ([3d26470](https://github.com/abn/coprctl/commit/3d26470439d281d2da88960f9128fd92aa706873))
* default project list to the current user ([1b72564](https://github.com/abn/coprctl/commit/1b725645231f3fd68e9790e7966b2a85c0a74814))
* **integration:** match hook by webhook destination and poll ping ([cc52ad6](https://github.com/abn/coprctl/commit/cc52ad640898f9cea549543903bab20b894bd851))
* make local SRPM build and upload work end to end ([cfda48c](https://github.com/abn/coprctl/commit/cfda48c81d198ecb27b93b6c3a0325fcaaca3957))
* make stdin read and container mounts portable ([daff57d](https://github.com/abn/coprctl/commit/daff57d293a9abe498da4d49864cc4dc803ac207))
* reconcile project chroots via the verified project edit endpoint ([f1ac034](https://github.com/abn/coprctl/commit/f1ac0348863b6b2cd2be20f90744fbc8dd4b6645))
* support side-repo dirs and release-please specs in local builds ([19b06ba](https://github.com/abn/coprctl/commit/19b06bac9b6f1a58255c3b2986bb794d88026ae8))


### Documentation

* add ADRs for implemented decisions and drop repeated titles ([c858d28](https://github.com/abn/coprctl/commit/c858d28a26b9a3a9427061041a7a7065a18f9ff7))
* add project README ([9494d32](https://github.com/abn/coprctl/commit/9494d32df9a70b988e3474bc53a6cbd8b56a38bb))
* add release process and sync spec version with release-please ([27f6ce2](https://github.com/abn/coprctl/commit/27f6ce25029fa8b8581152df2d3ea9dcfff494d5))
* complete the wiki to reflect status quo ([e2a0e8e](https://github.com/abn/coprctl/commit/e2a0e8e34cb5baaa01ed674a8afca827ffead1aa))
* correct config library in ADR 0001 ([c00e52a](https://github.com/abn/coprctl/commit/c00e52a547991ca677db13d0af2c2624b2acc88f))
* point to upstream Copr source as ground truth ([ef9edf5](https://github.com/abn/coprctl/commit/ef9edf54c4e0d21d29be7559b840d10d66519817))
* record chroot lifecycle and retire workflows ([d088658](https://github.com/abn/coprctl/commit/d0886581e3e6c657e2eca615fb763e3c369c6f5a))
* record completion of all project waves ([4e6b643](https://github.com/abn/coprctl/commit/4e6b643cc422bf60dadd309a14db0e421c85b9a6))
* record cross-platform release decision ([f4a4695](https://github.com/abn/coprctl/commit/f4a4695a7e664bcc58bb72577d292f92e7721549))
* scope the wiki log to knowledge-base evolution ([55f4590](https://github.com/abn/coprctl/commit/55f45900381aef723fcb3a253cf8707e00cbc065))
* standardize project and binary name on coprctl ([af7d0ca](https://github.com/abn/coprctl/commit/af7d0ca23f5e1801db88d2197b87b95de1922183))


### Build & Packaging

* set project homepage to coprctl.abn.is ([ebdf817](https://github.com/abn/coprctl/commit/ebdf817bf8831243105259bb477720dc90615f93))


### Continuous Integration

* add GitHub Actions, release-please, and dependabot ([efc644b](https://github.com/abn/coprctl/commit/efc644bbd4337b85410fb57918eb2b7ba6f3fac7))
* verify builds and tests across release platforms ([aa0606c](https://github.com/abn/coprctl/commit/aa0606cb69c51c589ebf137151b8c1002be3b550))
