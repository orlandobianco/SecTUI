# Changelog

## [0.2.0](https://github.com/orlandobianco/SecTUI/compare/v0.1.12...v0.2.0) (2026-03-05)


### ⚠ BREAKING CHANGES

* migrated from charmbracelet/bubbletea v1 to charm.land/bubbletea/v2 and charmbracelet/lipgloss v1 to charm.land/lipgloss/v2.

### Features

* upgrade to Bubble Tea v2 and Lipgloss v2 with security command center overview ([ee8404e](https://github.com/orlandobianco/SecTUI/commit/ee8404e266ebe9cc6851a353e55fe75b0de30a01))

## [0.1.12](https://github.com/orlandobianco/SecTUI/compare/v0.1.11...v0.1.12) (2026-03-05)


### Bug Fixes

* persistent background jobs with external process and streaming output ([304e35f](https://github.com/orlandobianco/SecTUI/commit/304e35f0c32b4f2dff0fa4ece6193c3c534ac4e8))

## [0.1.11](https://github.com/orlandobianco/SecTUI/compare/v0.1.10...v0.1.11) (2026-03-05)


### Features

* dynamic start/stop actions and sidebar badge refresh for tool management ([2135807](https://github.com/orlandobianco/SecTUI/commit/213580706c5b781e860f2d8c0286868ed5d937f9))
* structured output formatting and scrollable result view for tool actions ([a76e0cf](https://github.com/orlandobianco/SecTUI/commit/a76e0cfdb5b9b0c3f80824810870557274c92423))

## [0.1.10](https://github.com/orlandobianco/SecTUI/compare/v0.1.9...v0.1.10) (2026-03-05)


### Features

* add tool management UI with 4-panel layout ([d502eca](https://github.com/orlandobianco/SecTUI/commit/d502ecaf0e815fa65f08d4d595733430d9bb5886))

## [0.1.9](https://github.com/orlandobianco/SecTUI/compare/v0.1.8...v0.1.9) (2026-03-05)


### Features

* dynamic footer with contextual key hints per view ([b3210af](https://github.com/orlandobianco/SecTUI/commit/b3210afc188484112e630fb67a459e3dfd29a286))

## [0.1.8](https://github.com/orlandobianco/SecTUI/compare/v0.1.7...v0.1.8) (2026-03-05)


### Bug Fixes

* highlight focused pane with border color and cursor style ([5fc8893](https://github.com/orlandobianco/SecTUI/commit/5fc8893579e8a712a7e7d36d19402a2d29f8dd3e))

## [0.1.7](https://github.com/orlandobianco/SecTUI/compare/v0.1.6...v0.1.7) (2026-03-05)


### Bug Fixes

* rewrite SecStore as compact list layout ([c5fd923](https://github.com/orlandobianco/SecTUI/commit/c5fd923d0ddd9b216f8bf2bf65ebfff29038f071))

## [0.1.6](https://github.com/orlandobianco/SecTUI/compare/v0.1.5...v0.1.6) (2026-03-05)


### Bug Fixes

* SecStore card layout, adaptive resize, highlight, and Tab conflict ([b535b3c](https://github.com/orlandobianco/SecTUI/commit/b535b3c716d1ccfa4d09d7fc0ad24e0b7069d385))

## [0.1.5](https://github.com/orlandobianco/SecTUI/compare/v0.1.4...v0.1.5) (2026-03-05)


### Features

* add filesystem module for checking file permissions and security ([56741bd](https://github.com/orlandobianco/SecTUI/commit/56741bda63633a614e0b839cb6bada67af34086f))
* add SecStore with tool registry, detection, and install UI ([3351281](https://github.com/orlandobianco/SecTUI/commit/3351281e6f72e49bcddb6b16687eb25cf68b6b5e))
* implement report generation in JSON and Markdown formats ([56741bd](https://github.com/orlandobianco/SecTUI/commit/56741bda63633a614e0b839cb6bada67af34086f))


### Bug Fixes

* enhance user module to handle empty password checks with fixes ([56741bd](https://github.com/orlandobianco/SecTUI/commit/56741bda63633a614e0b839cb6bada67af34086f))

## [0.1.4](https://github.com/orlandobianco/SecTUI/compare/v0.1.3...v0.1.4) (2026-03-05)


### Bug Fixes

* **tui:** fix height rendering on Arch Linux ([fedd08d](https://github.com/orlandobianco/SecTUI/commit/fedd08d44388b7a4e912e0f1c81e3c89935c3570))

## [0.1.3](https://github.com/orlandobianco/SecTUI/compare/v0.1.2...v0.1.3) (2026-03-05)


### Bug Fixes

* **install:** update version resolution to fetch the most recent tag ([70ee896](https://github.com/orlandobianco/SecTUI/commit/70ee8963e9e5d9897597ea7e68e0e4fa989b49d5))

## [0.1.2](https://github.com/orlandobianco/SecTUI/compare/v0.1.1...v0.1.2) (2026-03-05)


### Features

* **core:** add unit tests for configuration loading and score calculation ([823a197](https://github.com/orlandobianco/SecTUI/commit/823a197f6af1f30b558311713b9193cb5ba628a7))
* **modules:** implement unit tests for kernel and SSH module functionalities ([823a197](https://github.com/orlandobianco/SecTUI/commit/823a197f6af1f30b558311713b9193cb5ba628a7))
* **tui:** add help overlay with keyboard shortcuts and navigation instructions ([823a197](https://github.com/orlandobianco/SecTUI/commit/823a197f6af1f30b558311713b9193cb5ba628a7))

## [0.1.1](https://github.com/orlandobianco/SecTUI/compare/v0.1.0...v0.1.1) (2026-03-05)


### Features

* add core configuration, platform detection, and security modules ([d02ce76](https://github.com/orlandobianco/SecTUI/commit/d02ce76c7dd5c23d2f6f747669cdbb96d112d4f6))
* add initial README.md with project overview, installation, and usage instructions ([ad46203](https://github.com/orlandobianco/SecTUI/commit/ad46203ce0a237fdda9b34ed69592af3489ae1b3))
* **locales:** embed locale files for internationalization support ([1237494](https://github.com/orlandobianco/SecTUI/commit/123749460c00015db996432094d55b8609ccdd6c))
* **release:** update release configuration and add manifest file ([90cae7b](https://github.com/orlandobianco/SecTUI/commit/90cae7bb1ce59e33ddcbdf2e283a6a6a9137a641))
* remove specific paths from GitHub Pages deployment trigger ([1f7c8df](https://github.com/orlandobianco/SecTUI/commit/1f7c8df150f661978d06f945b7946c2e871a8cc6))
* **tui:** add root privilege check for applying fixes and enhance fix flow ([75d50ea](https://github.com/orlandobianco/SecTUI/commit/75d50eae872c03816a5ffe7a46102e38053c6f9c))
* **tui:** add scanner view for security scan progress and findings display ([1237494](https://github.com/orlandobianco/SecTUI/commit/123749460c00015db996432094d55b8609ccdd6c))
* **tui:** create sidebar for navigation and module selection ([1237494](https://github.com/orlandobianco/SecTUI/commit/123749460c00015db996432094d55b8609ccdd6c))
* **tui:** establish theme styles for consistent UI appearance ([1237494](https://github.com/orlandobianco/SecTUI/commit/123749460c00015db996432094d55b8609ccdd6c))
* **tui:** implement fix confirmation flow and apply fixes functionality ([c58a200](https://github.com/orlandobianco/SecTUI/commit/c58a200eb4039ae664237b9d04c4814c4c65e55b))
* **tui:** improve i18n support for finding titles and details in ModuleView ([98c5d1a](https://github.com/orlandobianco/SecTUI/commit/98c5d1a6a54f9a6cec7cdc687bba3e59af27174f))
* update release workflow and improve module method formatting ([cd7f67b](https://github.com/orlandobianco/SecTUI/commit/cd7f67b496b49c6b81b09330dbb0f67c06a4780f))

## [1.2.0](https://github.com/orlandobianco/SecTUI/compare/v1.1.0...v1.2.0) (2026-03-05)


### Features

* **tui:** add root privilege check for applying fixes and enhance fix flow ([75d50ea](https://github.com/orlandobianco/SecTUI/commit/75d50eae872c03816a5ffe7a46102e38053c6f9c))

## [1.1.0](https://github.com/orlandobianco/SecTUI/compare/v1.0.0...v1.1.0) (2026-03-05)


### Features

* **tui:** implement fix confirmation flow and apply fixes functionality ([c58a200](https://github.com/orlandobianco/SecTUI/commit/c58a200eb4039ae664237b9d04c4814c4c65e55b))

## 1.0.0 (2026-03-05)


### Features

* add core configuration, platform detection, and security modules ([d02ce76](https://github.com/orlandobianco/SecTUI/commit/d02ce76c7dd5c23d2f6f747669cdbb96d112d4f6))
* add initial README.md with project overview, installation, and usage instructions ([ad46203](https://github.com/orlandobianco/SecTUI/commit/ad46203ce0a237fdda9b34ed69592af3489ae1b3))
* **locales:** embed locale files for internationalization support ([1237494](https://github.com/orlandobianco/SecTUI/commit/123749460c00015db996432094d55b8609ccdd6c))
* remove specific paths from GitHub Pages deployment trigger ([1f7c8df](https://github.com/orlandobianco/SecTUI/commit/1f7c8df150f661978d06f945b7946c2e871a8cc6))
* **tui:** add scanner view for security scan progress and findings display ([1237494](https://github.com/orlandobianco/SecTUI/commit/123749460c00015db996432094d55b8609ccdd6c))
* **tui:** create sidebar for navigation and module selection ([1237494](https://github.com/orlandobianco/SecTUI/commit/123749460c00015db996432094d55b8609ccdd6c))
* **tui:** establish theme styles for consistent UI appearance ([1237494](https://github.com/orlandobianco/SecTUI/commit/123749460c00015db996432094d55b8609ccdd6c))
* **tui:** improve i18n support for finding titles and details in ModuleView ([98c5d1a](https://github.com/orlandobianco/SecTUI/commit/98c5d1a6a54f9a6cec7cdc687bba3e59af27174f))
* update release workflow and improve module method formatting ([cd7f67b](https://github.com/orlandobianco/SecTUI/commit/cd7f67b496b49c6b81b09330dbb0f67c06a4780f))
