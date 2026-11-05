# Source context — separate from the proposed wizard

Read-only source anchors inspected on 2026-11-05. Local uncommitted application changes were present. This is limited compatibility context, not an existing-app contract extraction or proof of the proposed wizard. New UI, random configuration, entropy policy, persistence and stream contracts are proposed. No application scripts, builds or runtime tests were executed.

- [internal/app/app.go:66](../../../../../internal/app/app.go#L66): Current CLI rejects positional arguments; init needs separate dispatch. Full-file SHA-256 `a736c80470720588e85d13e39da8d143540080b3cb42894e2bd99323a5d9775e`.
- [internal/config/config.go:54](../../../../../internal/config/config.go#L54): Existing config discovery honors XDG_CONFIG_HOME then HOME. Full-file SHA-256 `dd7e2c90a3e151ff77dacd0520cbbc3966ff8d4890af762842e37124c2c2a81e`.
- [internal/config/config.go:24](../../../../../internal/config/config.go#L24): Existing on-disk model includes face, format, color and styles. Full-file SHA-256 `dd7e2c90a3e151ff77dacd0520cbbc3966ff8d4890af762842e37124c2c2a81e`.
- [internal/face/face.go:11](../../../../../internal/face/face.go#L11): Existing face options have preset/seed/state and no random field. Full-file SHA-256 `250b2ed4825fea9d7393360a8d90d97f03036624fe0aba236a53adeea7658761`.
- [internal/face/face.go:18](../../../../../internal/face/face.go#L18): Existing face validation rejects preset+seed. Full-file SHA-256 `250b2ed4825fea9d7393360a8d90d97f03036624fe0aba236a53adeea7658761`.
- [internal/face/face.go:32](../../../../../internal/face/face.go#L32): Absent preset/seed currently uses stable smol seed. Full-file SHA-256 `250b2ed4825fea9d7393360a8d90d97f03036624fe0aba236a53adeea7658761`.
- [internal/face/face.go:54](../../../../../internal/face/face.go#L54): Preset identifiers come from upstream table. Full-file SHA-256 `250b2ed4825fea9d7393360a8d90d97f03036624fe0aba236a53adeea7658761`.
- [internal/face/face.go:63](../../../../../internal/face/face.go#L63): Expression identifiers come from the current state API. Full-file SHA-256 `250b2ed4825fea9d7393360a8d90d97f03036624fe0aba236a53adeea7658761`.
- [internal/modules/module.go:60](../../../../../internal/modules/module.go#L60): Current names include face plus nine system modules. Full-file SHA-256 `98840f27b02919e714511c010ff8f6545d61d1b34aeddad1f962af0cb4a710c2`.
