# Patent landscape: autonomous lead research agent

Scanned 2026-09-12 against this repo's feature set: agentic planning loop,
lead discovery via API, lead scoring, email verification, dedup / entity
resolution, outreach-message generation, CSV import/export.

Sources: the local patent library in `izdrail/maicoach`
(`research/patents/library`, 2,902 full-text CPC G06 publications from the
last ~150 days) searched first, then a wider Google Patents search for
lead-scoring, email-discovery and email-verification patents.

This is a technical landscape review, not legal advice. "Relevant" means the
patent's claims overlap with things this project does or plausibly will do;
freedom-to-operate needs a patent attorney before commercial reliance.

## Directly relevant (worth reading in full)

### US11775912B2 - Systems and methods for real-time lead grading (Leadscorz Inc)
- https://patents.google.com/patent/US11775912B2/en
- Receives a lead from a vendor, converts attribute values to benchmark
  values, assigns the lead to a pre-determined cluster, grades in real time.
- Overlap: `internal/scoring` and `internal/leads/score.go` do attribute-based
  lead grading at ingest time. Read the independent claims before adding
  "benchmark normalization against vendor populations" - that normalization
  step is the heart of their claims, not generic scoring.

### US11544582B2 - Predictive modelling to score customer leads (Ambertag Inc)
- https://patents.google.com/patent/US11544582B2/en
- Automated model selection for lead scoring: variable selection, feature-set
  selection, training-data sampling, iterative local+global optimization.
- Overlap: low today (scoring here is rule-based), high if an ML scorer is
  added. If we train models per objective, avoid their auto-model-selection
  pipeline shape.

### US10394834B1 / US10860593 - Ranking leads based on given characteristics (Massachusetts Mutual)
- https://patents.google.com/patent/US10394834B1/en
- Ranking and appraising lead quality inside a lead auction/exchange
  architecture.
- Overlap: ranking yes, auction/exchange no. Their claims are tied to the
  marketplace architecture, which this project does not build.

### US20140149178A1 - Lead scoring (ICE Mortgage Technology)
- https://patents.google.com/patent/US20140149178A1/en
- Score sales leads by attributes + likelihood of desired outcome, with the
  scoring model updated from realized outcomes.
- Overlap: the "update the model from outcomes" loop is the claimable core.
  Our scoring is static rules with no outcome feedback; adding an outcome
  feedback loop should be designed around their claims.

### US8495151B2 - Methods and systems for determining email addresses (eGrabber Inc)
- https://patents.google.com/patent/US8495151B2/en
- Given partial contact info, determine missing email fields by searching
  the internet and generating candidate addresses.
- Overlap: email *discovery* (a natural next tool for this agent - the
  LeadsCaptain client already accepts `emails[]` fallbacks). Old patent
  (priority 2002), but read before building an email-finder tool.

### US9917806B2 - Method and system for email address validation (IBM)
- https://patents.google.com/patent/US9917806B2/en
- Detect an erroneous recipient address and recommend a corrected one.
- Overlap: `internal/emailvalidator` verifies but does not suggest
  corrections. A "did you mean" feature for catch-all/typo domains would sit
  close to this patent.

## From the local library (maicoach/research/patents/library)

### DE202026101185U1 - Agent-based AI reconciliation for CRM with golden records (Sanjoy Mukherjee)
- https://patents.google.com/patent/DE202026101185U1/en
- Agent-based AI orchestration engine + relational warehousing + MDM
  "golden record" management + duplicate/mismatch detection + human-in-the-loop
  resolution console + audit/continuous learning.
- Overlap: the closest single patent to this project's shape. Our dedup is
  `INSERT OR IGNORE` on external key plus email-existence checks; a golden-record
  merge flow with a review console would be the natural upgrade - and is
  exactly what this Gebrauchsmuster claims. German utility model, DE-only.

### CN121902212B - Three-party efficient private record linking (University of Jinan)
- https://patents.google.com/patent/CN121902212B/en
- Privacy-preserving record linkage across large-scale datasets.
- Overlap: relevant if we ever merge lead records with a partner's data
  without pooling raw PII. CN-only grant.

### CN122066453A - GEO-oriented marketing content generation with a fact graph (Shanghai Wangmai)
- https://patents.google.com/patent/CN122066453A/en
- Generates marketing copy constrained by a "fact map" of product facts,
  rules and validity windows to stop hallucinated claims.
- Overlap: `generateIntros` in the agent writes outreach copy with no fact
  grounding. Grounding personalization in a per-lead fact record is a good
  design regardless of the patent; the patent claims the fact-map constraint
  mechanism specifically. CN application, pending.

### KR20260070355A - Embedded AI-agent workflow instructions in web pages (Han Won-pyo)
- https://patents.google.com/patent/KR20260070355A/en
- Structured "AI manifest" embedded in pages so agents skip DOM scraping;
  includes a trust-registry defense against prompt injection.
- Overlap: tangential to lead research, but its prompt-injection defense is
  relevant to any future web-browsing tool the agent gets. KR application,
  pending.

## Gaps in the library

The library is a raw CPC G06 dump of the last ~150 days and holds nothing on
lead scoring, email discovery or email verification - the wider Google Patents
search above fills that gap. If this matters long-term, the fetch script
(`maicoach/scripts/fetch_latest_patents.py`) could run with CPC queries
targeted at G06Q 30/02 (marketing) and H04L 51 (messaging) instead of all
of G06.

## Practical takeaways for this codebase

1. Rule-based scoring (what we ship) is well clear of the scoring patents;
   the risky step is adding *learned, outcome-fed, auto-selected* models.
2. A golden-record merge console is the most valuable - and most
   patent-adjacent - roadmap item. Design it with DE202026101185U1 open.
3. Keep email verification to verify-only; "suggest a correction" brushes
   against IBM's US9917806B2.
4. Ground outreach generation in stored lead facts (good engineering, and it
   avoids CN122066453A's fact-map claim shape by using the lead record itself
   rather than a marketing fact graph).
