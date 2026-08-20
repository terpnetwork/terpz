# lean-cw-ffi

C ABI around Commonware `simplex::Engine` for Lean AppState.

- `Automaton.propose` / `verify` → Go `PrepareProposal` / `ProcessProposal`
- `Reporter` + finalization → Go `FinalizeBlock` / `Commit`
- Epoch = Lean period / daily keys (participant set frozen for the engine epoch)
- Dummy DSTW still fails in Go Process (`prover_id=2`, `curve_id=5`)
- JOIN/LEAV remain LNPR subjects

Start with `LEAN_CONSENSUS=commonware`. Default remains Comet.
