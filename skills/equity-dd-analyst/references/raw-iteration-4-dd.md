# Raw Iteration 4 DD Output

**Simple Conclusion**

This is a four-way comparison of very different “data centre” exposures. Research view, not personal financial advice: **SXE.AX has the best risk/reward today**, **IFT.AX is the highest-quality data-centre exposure**, **SKS.AX is the highest-beta/speculative contractor**, and **VNT.AX is a good infrastructure services business but a weak pure data-centre proxy**.

- **Best risk/reward:** **SXE.AX** - direct contractor exposure to data centres, net cash, raised FY26 EBITDA guidance, and less valuation stretch than SKS.
- **Best quality:** **IFT.AX** - CDC is the cleanest and highest-quality underlying asset, with >1GW contracted capacity after the new 555MW contract, but Infratil’s public valuation already discounts a lot.
- **Most speculative:** **SKS.AX** - strong data-centre backlog and momentum, but the stock is pricing rapid execution almost perfectly.
- **Wait/avoid:** **SKS.AX at current levels** - I would want either a pullback or more proof that 10% PBT margins survive the larger FY27 workload.
- **Confidence:** **Medium-high** - company-filed evidence is strong for contracts/backlog/guidance; confidence is capped because MarketPal financial TTM dates appear inconsistent for some tickers and VNT does not disclose data-centre revenue separately.

**Ranking Table**

| Rank | Ticker | Exposure Type | Core Thesis | Key Risk | Verdict |
|---:|---|---|---|---|---|
| 1 | SXE.AX | Contractor/services provider | Net-cash electrical contractor with direct data-centre wins, $710m order book, and FY26 EBITDA guidance of at least $72m | Project execution, margin volatility, residual WestConnex hit | Best risk/reward |
| 2 | IFT.AX | Infrastructure investor | 49.72% CDC stake gives high-quality, long-duration contracted data-centre exposure | Heavy capex, funding, valuation already high | Best quality, not cheapest |
| 3 | VNT.AX | Network/services adjacent | Large resilient services platform, $22.1bn work in hand, credible digital-infrastructure strategy | Data-centre economics not quantified | Good stock, weak DC proxy |
| 4 | SKS.AX | Contractor/services provider | Most direct small/mid-cap data-centre contractor, $350m work on hand, $210m MEL02A contract | Valuation, scale-up, working-capital/bonding capacity | Speculative/wait |

**Scorecard**

Scores are 1-5, where 5 is best and execution risk means lower risk. Overall uses 30% exposure purity, 25% valuation, 20% financial resilience, 15% growth visibility, 10% execution risk.

| Ticker | Exposure Purity | Growth Visibility | Balance Sheet | Valuation | Execution Risk | Downside Protection | Overall |
|---|---:|---:|---:|---:|---:|---:|---:|
| IFT.AX | 5.0 | 5.0 | 3.0 | 2.5 | 3.0 | 3.0 | 3.8 |
| SXE.AX | 3.5 | 4.0 | 4.5 | 3.0 | 3.5 | 3.5 | 3.7 |
| VNT.AX | 2.0 | 3.5 | 4.0 | 4.0 | 4.0 | 4.0 | 3.3 |
| SKS.AX | 4.0 | 4.0 | 4.0 | 1.5 | 2.5 | 2.0 | 3.2 |

**MarketPal-Derived Facts**

MarketPal data used: `capabilities`, `ticker fundamentals`, `ticker financials`, `ticker events`, `ticker insiders`, `ticker ownership`, and `ticker profile`, run from source-built `go run ./cmd/mpal` on 10 May 2026. No MarketPal insider transactions or ownership-flow events were returned for the four tickers over the lookback window.

| Ticker | Price / Market Cap | EV | Valuation Snapshot | MarketPal Signal Notes |
|---|---:|---:|---|---|
| SXE.AX | A$4.09 / A$1.09bn | A$1.04bn | Forward P/E 24.5x; FY26 company-guided EV/EBITDA roughly 14.4x using at least A$72m EBITDA | QVM 82.1, momentum 100, 1Y return 127.8%; Markov weekly state flat |
| SKS.AX | A$8.04 / A$927m | A$878m | Forward P/E 27.0x; EV/FY26 guided PBT about 25.8x | QVM 76.1, momentum 100, 1Y return 416%; Markov flat |
| IFT.AX | A$12.40 / A$12.37bn | A$20.09bn | P/E 62x; EV/FCF negative on MarketPal data | QVM 33.1, momentum 72.5; 1Y return 17.4%; Markov flat |
| VNT.AX | A$5.72 / A$4.70bn | A$5.41bn | P/E 17.9x, forward P/E 15.6x, EV/EBIT 12.4x, FCF yield 5.7% | QVM 78.1; Markov flat |

**Company-Filed Evidence Matrix**

| Ticker | Exposure | DC Revenue / Backlog / Capacity | Balance Sheet / Cash Flow | Valuation Read | Catalyst | Downside Case | Evidence Confidence |
|---|---|---|---|---|---|---|---|
| IFT.AX | 49.72% owner of CDC via infrastructure holding company | CDC’s 555MW contract takes contracted capacity above 1GW; CDC pipeline 2.9GW at Mar-26; FY28 EBITDAF expected >A$1bn | CDC has Baa2 rating and A$3.9bn cash/undrawn borrowings; Infratil says no further equity needed for new contract | Look-through CDC stake last valued A$6.95bn at Dec-25 before 555MW step-up | CDC FY26/FY27 build delivery and 26 May FY26 result | Capex, power/grid, debt cost, valuation sensitivity | High |
| SXE.AX | Electrical/data-centre contractor | DigiCo SYD1 work, NEXTDC Artarmon contributor, “unprecedented” CY26 DC tender pipeline; $710m order book | H1 FY26 cash A$58.8m, debt free, A$53.2m bonding headroom | ~14x FY26 guided EBITDA; cleaner than SKS, but not cheap | More data-centre tender wins and bonding release | Fixed-price project risk, labour, WestConnex confidence overhang | High |
| SKS.AX | Electrical/digital-infrastructure contractor | A$210m MEL02A contract for 126MW hyperscale facility; A$350m work on hand, A$240m into FY27 | A$20m bank-guarantee facility increase to A$52m per MarketPal event; FY25 cash A$32.5m | Rich: EV/FY26 guided PBT about 26x; MarketPal fwd P/E 27x | Conversion of hyperscale pipeline and NSW expansion | Margin slippage, working capital, valuation compression | High |
| VNT.AX | Essential infrastructure services, indirect DC services | Data-centre site prep, fit-out and O&M strategy; addressable outsourced DC services market cited at A$2.6bn to A$5.9bn by FY30; no disclosed DC revenue | FY25 revenue A$6.1bn, EBITDA A$532m, work in hand A$22.1bn, OCF conversion 93.6% | Cheapest and most defensive, but least pure | Digital infrastructure push, buyback, FY26 NPATA growth guidance | DC thesis may remain immaterial to group earnings | Medium |

**Company Notes**

**IFT.AX - Infratil**
Company-filed facts: CDC signed a 555MW contract with a high-end US investment-grade customer, delivered across FY28-FY29, lifting contracted capacity above 1GW. Infratil owns 49.7%/49.72% of CDC depending on source date. CDC expects FY28 EBITDAF above A$1bn and about A$2bn annualised EBITDAF when 1GW is fully deployed. CDC Australia has a Baa2 stable rating, and Infratil says the new contract does not require further shareholder equity.
View: this is the best asset quality and cleanest data-centre exposure, but public shareholders buy a diversified holdco with capital intensity and valuation sensitivity. Bull case is long-duration contracted hyperscale demand, funding access, and mid-teens equity returns. Bear case is build delay, grid/power constraints, higher debt cost, or a lower CDC valuation discount rate.

**SXE.AX - Southern Cross Electrical Engineering**
Company-filed facts: H1 FY26 revenue was A$349.1m, underlying EBITDA A$35.4m, gross margin 18.9%, order book A$710m, cash A$58.8m, debt free, with FY26 underlying EBITDA guidance raised to at least A$72m. Data-centre awards include DigiCo SYD1 work and Sydney hyperscale packages via Heyday.
View: SXE is the most balanced listed contractor exposure. It has direct exposure, stronger financial resilience than SKS, and proven scale, though contract mix and execution still matter. Bull case is conversion of the CY26 data-centre tender pipeline into a multi-year order book. Bear case is margin normalisation after a strong H1 and project disputes.

**SKS.AX - SKS Technologies**
Company-filed facts: SKS expanded its Hickory/MEL02A data-centre contract to A$210m for a 126MW facility, expects completion in 3Q27, and reported A$350m work on hand with A$240m flowing into FY27. February guidance was FY26 revenue A$340m and 10% PBT margin, implying A$34m PBT.
View: SKS has the most explosive thematic torque, but the current market cap is asking investors to underwrite both rapid revenue scale and margin stability. Bull case is continued hyperscale awards and repeat business. Bear case is working-capital drag, execution stretch, or a derating if FY27 margins slip.

**VNT.AX - Ventia**
Company-filed facts: FY25 revenue was A$6.1bn, EBITDA A$532.1m, NPATA A$257.6m, work in hand A$22.1bn, and FY26 NPATA guidance is 7-10% growth. Investor Day materials identify data-centre site preparation, internal fit-out and O&M as digital-infrastructure growth pillars.
View: VNT is the best defensive valuation among the four, but the data-centre link is indirect and unquantified. Bull case is capital-light growth in digital infrastructure plus buyback support. Bear case is that DC remains too small to change the group multiple.

**Cross-Company Read**

IFT owns the superior economic exposure; SXE and SKS sell labour, electrical systems and delivery capability into the build cycle; VNT services adjacent infrastructure. For a PM trying to express a data-centre view, the main decision is **quality versus price**. IFT gives long-duration contracted capacity but with holdco and capex complexity. SXE gives direct contractor upside with a better balance sheet. SKS gives the most torque but the least margin of safety. VNT is a lower-risk infrastructure compounder, not a clean data-centre trade.

**What Would Change The View**

1. SKS reporting FY27 backlog conversion with cash conversion intact and PBT margin still near 10%.
2. IFT/CDC confirming funding terms, power delivery and customer activation timing for the 555MW contract.
3. SXE winning additional named hyperscale packages while maintaining bonding headroom and H2 FY26 margin quality.

**Source Notes**

Company-filed / primary sources: Infratil/NZX CDC 555MW announcement, 6 May 2026: https://www.nzx.com/announcements/472137; Infratil CDC contract presentation, 5 May 2026: https://infratil.com/news/cdc-signs-555mw-data-centre-contract-with-us-customer/cdc-contract-update-growing-beyond-1gw/; Infratil interim results, 13 Nov 2025: https://infratil.com/news/interim-results-for-the-period-ended-30-september-2025/; CDC valuation, 31 Dec 2025: https://infratil.com/news/cdc-independent-valuation-31-december-2025/cdc-independent-valuation-31-december-2025/; SKS A$210m contract release, 17 Apr 2026: https://announcements.asx.com.au/asxpdf/20260417/pdf/06yl2p0dc3cj60.pdf; SKS February guidance release via MarketPal/ASX source; SXE H1 FY26 release: https://cdn-api.markitdigital.com/apiman-gateway/ASX/asx-research/1.0/file/2924-03057594-6A1312508%26v%3Dundefined; SXE A$90m data-centre/rail awards: https://investorpa.com/announcement-pdf/20251216/235467.pdf; SXE A$75m awards: https://data-api.marketindex.com.au/api/v1/announcements/XASX%3ASXE%3A6A1311435/pdf/inline/contract-awards-totalling-75m; SXE March 2026 roadshow: https://company-announcements.afr.com/asx/sxe/9dc7e293-1b33-11f1-b79e-3e31e35c7fba.pdf; Ventia FY25 media release: https://investorpa.com/announcement-pdf/20260219/258256.pdf; Ventia Investor Day: https://company-announcements.afr.com/asx/vnt/64e7d5a1-473d-11f1-a82c-cad0a84b142a.pdf.

MarketPal-derived facts are from source-built `mpal` as of 9-10 May 2026. Data gap: MarketPal returned current valuation/event/profile data, but some `ticker financials` TTM period dates looked stale or mislabeled; high-impact financial figures above therefore rely on company filings where available.
