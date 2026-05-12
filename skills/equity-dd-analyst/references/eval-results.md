# Equity DD Analyst Eval Results

This file records cold-start validation runs used to refine the skill. It is a maintainer artifact, not required reading for normal DD execution.

## Iteration 1 - Baseline Skill

- **Cold-start runner:** Banach
- **Judge:** Turing
- **Average score:** 4.29/5
- **Result:** PASS
- **Scores:** source discipline 4, exposure classification 5, financial analysis 4, comparison consistency 4, investment debate 4, conclusion usefulness 5, MarketPal integration 4.
- **Main weaknesses:** valuation was not institutional-grade, score weighting was unclear, MarketPal command usage was not explicit enough, IFT needed look-through math, and contractor names needed more working-capital, bonding, and margin analysis.
- **Changes made:** added valuation requirements, holding-company look-through math, contractor backlog/cash-conversion/bonding analysis, and MarketPal/source-labeling instructions.

## Iteration 2 - Valuation And Mechanics Added

- **Cold-start runner:** Faraday
- **Judge:** Gibbs
- **Average score:** 4.29/5
- **Result:** PASS
- **Scores:** source discipline 4, exposure classification 5, financial analysis 4, comparison consistency 4, investment debate 4, conclusion usefulness 5, MarketPal integration 4.
- **Main weaknesses:** SKS source trail still leaned too secondary, IFT needed deeper NAV/funding/return-on-capital math, contractor analysis could go further, and overall weighting was not fully auditable.
- **Changes made:** added primary-source-before-secondary rule, source-pack tracking, evidence confidence, explicit thematic weighting, and a uniform evidence matrix.

## Iteration 3 - Evidence Matrix And Weighting Added

- **Cold-start runner:** Helmholtz
- **Judge:** Newton
- **Average score:** 4.43/5
- **Result:** PASS
- **Scores:** source discipline 4.0, exposure classification 5.0, financial analysis 4.5, comparison consistency 4.5, investment debate 4.0, conclusion usefulness 5.0, MarketPal integration 4.0.
- **Strongest improvements:** clear ranked conclusion, correct contractor/investor/adjacent exposure classification, consistent scorecard and evidence matrix, visible evidence confidence, MarketPal facts separated from company-filed facts, and explicit handling of contaminated MarketPal event data.
- **Remaining weaknesses:** no direct source links in the final report, limited peer valuation context, no detailed sensitivity table, limited IFT/CDC debt-cost stress, limited contractor contract-type and margin-range detail, and VNT data-center immateriality remained mostly inferred from non-disclosure.
- **Judge conclusion:** further iteration was unlikely to produce material skill-level improvement because the report already cleared the rubric comfortably and the remaining issues were mostly source-linking, output-length, unavailable data, or external tool constraints.

## Iteration 4 - Updated mpal Capabilities Integrated

- **Cold-start runner:** Kant
- **Judge:** Boole
- **Average score:** 4.71/5
- **Result:** PASS
- **Scores:** source discipline 4.5, exposure classification 5.0, financial analysis 4.5, comparison consistency 4.75, investment debate 4.5, conclusion usefulness 5.0, MarketPal integration 4.75.
- **Strongest improvements:** the report used the source-built `mpal` capability set, including `ticker fundamentals`, `ticker financials`, `ticker insiders`, `ticker ownership`, and profile evidence including Markov-style transition context when exposed; MarketPal facts were timestamped and isolated; stale or mislabeled MarketPal financial periods were flagged instead of blindly trusted; negative evidence from empty insider/ownership-flow checks was disclosed.
- **Remaining weaknesses:** granular inline citation density could improve, valuation still lacks full sensitivity tables and peer normalization, IFT/CDC funding stress remains lighter than a full model, contractor project-type/customer-concentration detail is limited, and VNT data-center immateriality remains inferred from disclosure gaps.
- **Judge conclusion:** further skill iteration is unlikely to produce material improvement under the current rubric. Remaining gaps are mostly data availability, output length, source extraction depth, and MarketPal/filing-tool coverage rather than prompt structure.

## Current Plateau Assessment

After the updated mpal capability integration, the practical plateau score is 4.71/5. The skill is now strong enough for senior-analyst-style DD with a simple conclusion. Further material gains likely require upstream tooling: filing/PDF retrieval with exact URLs and text extraction, normalized peer comps, debt schedules, segment data, data-center capacity datasets, transcript retrieval, and cleaner MarketPal financial period metadata.

## Iteration 5 - Valuation Stress And Inline Evidence Tightened

- **Cold-start runner:** Pauli
- **Judge:** Halley
- **Average score:** 4.75/5
- **Result:** PASS
- **Scores:** source discipline 4.75, exposure classification 5.00, financial analysis 4.75, comparison consistency 4.25, investment debate 4.75, conclusion usefulness 5.00, MarketPal integration 4.75.
- **Strongest improvements:** explicit valuation stress tests for all four names, stronger contractor mechanics, clearer primary/MarketPal/secondary source separation, better data-quality skepticism, and a highly decision-useful conclusion.
- **Remaining weaknesses:** weighted-score arithmetic mistakes in the generated scorecard, imperfect source access for some primary docs, thin peer-comp context, incomplete IFT funding/NAV bridge, and VNT data-center immateriality still inferred from non-disclosure.
- **Changes made after judge:** added a scorecard arithmetic QA instruction requiring agents to recalculate weighted overall scores and explain any override.
- **Judge conclusion:** the skill improved slightly from 4.71 to 4.75 and is effectively plateauing. Future material gains require better tooling/data extraction and calculation QA, not broad prompt rewrites.

## Current Plateau Assessment

After iteration 5, the practical plateau score is 4.75/5. The remaining limitations are mostly upstream data/tooling and deterministic calculation QA: direct filing/PDF retrieval, normalized peer comps, debt schedules, segment data, data-center capacity datasets, transcript retrieval, cleaner MarketPal financial period metadata, and ideally a deterministic scorecard calculator.
