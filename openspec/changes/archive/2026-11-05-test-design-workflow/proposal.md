# Test design workflow and the first three audit priorities

The user requested a bilingual layer/case/CI map, design review before implementation,
and implementation of the three highest-priority test-audit findings. Separate
technology from test boundaries, add traceable design records and enforce source/index
consistency. Run existing native document oracles in CI, separate manual tools and
make send diagnostics fixture-only, and compare actual RN/browser rendering against a
shared semantic fixture. Existing test/permission/signing gates remain required.

Design records were committed before test code. The initial inventory is explicitly
legacy structural triage. No release, installation or real-session test is included.
