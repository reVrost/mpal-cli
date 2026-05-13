<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Marketpal Trade Review {{.ID}}</title>
  <style>
    :root {
      color-scheme: light;
      --ink: #17202a;
      --muted: #5d6975;
      --line: #d8e0e7;
      --panel: #ffffff;
      --band: #f5f7f9;
      --head: #e9eef3;
      --trade: #0f766e;
      --skip: #9f1239;
      --watch: #9a5b00;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      background: var(--band);
      color: var(--ink);
      font: 14px/1.45 -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
    }
    main { max-width: 1280px; margin: 0 auto; padding: 28px 24px 40px; }
    header { margin-bottom: 18px; }
    h1 { margin: 0 0 10px; font-size: 24px; line-height: 1.2; letter-spacing: 0; }
    h2 { margin: 22px 0 10px; font-size: 15px; letter-spacing: 0; }
    .meta { display: flex; flex-wrap: wrap; gap: 8px; }
    .chip {
      display: inline-flex;
      align-items: center;
      gap: 6px;
      min-height: 28px;
      padding: 4px 9px;
      border: 1px solid var(--line);
      background: var(--panel);
      border-radius: 6px;
      color: var(--muted);
      white-space: nowrap;
    }
    .chip strong { color: var(--ink); font-weight: 650; }
    .summary {
      display: grid;
      grid-template-columns: repeat(3, minmax(0, 1fr));
      gap: 12px;
      margin: 16px 0;
    }
    .metric { background: var(--panel); border: 1px solid var(--line); border-radius: 6px; padding: 12px; }
    .metric span { display: block; color: var(--muted); font-size: 12px; }
    .metric strong { display: block; margin-top: 2px; font-size: 18px; }
    .table-wrap { overflow-x: auto; background: var(--panel); border: 1px solid var(--line); border-radius: 6px; }
    table { width: 100%; border-collapse: collapse; min-width: 1100px; }
    th {
      background: var(--head);
      color: #26323d;
      font-size: 12px;
      font-weight: 700;
      text-align: left;
      vertical-align: bottom;
      white-space: nowrap;
      padding: 9px 10px;
      border-bottom: 1px solid var(--line);
    }
    td { padding: 10px; border-bottom: 1px solid var(--line); vertical-align: top; }
    tr:last-child td { border-bottom: 0; }
    td.num, th.num { text-align: right; font-variant-numeric: tabular-nums; }
    .ticker { font-weight: 700; white-space: nowrap; }
    .decision { font-weight: 700; white-space: nowrap; }
    .decision-trade { color: var(--trade); }
    .decision-skip { color: var(--skip); }
    .decision-watch { color: var(--watch); }
    .decision-empty { color: var(--muted); }
    .read { min-width: 260px; color: #33414f; }
    .block { white-space: pre-wrap; background: var(--panel); border: 1px solid var(--line); border-radius: 6px; padding: 12px; }
    @media (max-width: 760px) {
      main { padding: 18px 12px 28px; }
      .summary { grid-template-columns: 1fr; }
      h1 { font-size: 21px; }
    }
  </style>
</head>
<body>
<main>
  <header>
    <h1>Marketpal Trade Review</h1>
    <div class="meta">
      <span class="chip">ID <strong>{{.ID}}</strong></span>
      <span class="chip">As of <strong>{{.AsOf}}</strong></span>
      <span class="chip">Strategy <strong>{{.StrategyID}}</strong></span>
      <span class="chip">Execution <strong>{{.ExecutionResult}}</strong></span>
    </div>
  </header>

  <section class="summary">
    <div class="metric"><span>Positions Reviewed</span><strong>{{len .Positions}}</strong></div>
    <div class="metric"><span>Trade Candidates</span><strong>{{.TradeCount}}</strong></div>
    <div class="metric"><span>Final Decision</span><strong>{{.FinalDecision}}</strong></div>
  </section>

  <section>
    <h2>Deterministic First Pass</h2>
    <div class="table-wrap">
      <table>
        <thead>
          <tr>
            <th>Ticker</th>
            <th>Trade?</th>
            <th>Decision</th>
            <th class="num">Score</th>
            <th>Role</th>
            <th>Intent</th>
            <th>Sizing</th>
            <th class="num">Raw Kelly</th>
            <th class="num">Frac Kelly</th>
            <th class="num">Kelly Target</th>
            <th class="num">Accepted %</th>
            <th class="num">Est. Value</th>
            <th>Binding</th>
            <th>Calibration</th>
            <th>Read</th>
          </tr>
        </thead>
        <tbody>
          {{range .Positions}}
          <tr>
            <td class="ticker">{{.Ticker}}</td>
            <td>{{.IsTrade}}</td>
            <td class="decision {{.DecisionClass}}">{{.Decision}}</td>
            <td class="num">{{.Score}}</td>
            <td>{{.Role}}</td>
            <td>{{.Intent}}</td>
            <td>{{.SizingMethod}}</td>
            <td class="num">{{.RawKelly}}</td>
            <td class="num">{{.FractionalKelly}}</td>
            <td class="num">{{.KellyTargetWeight}}</td>
            <td class="num">{{.AcceptedSizing}}</td>
            <td class="num">{{.EstimatedValue}}</td>
            <td>{{.BindingConstraint}}</td>
            <td>{{.CalibrationStatus}}</td>
            <td class="read">{{.Read}}</td>
          </tr>
          {{end}}
        </tbody>
      </table>
    </div>
  </section>

  {{if .AgentSummary}}<section><h2>Agent Summary</h2><div class="block">{{.AgentSummary}}</div></section>{{end}}
  {{if .HumanReasoning}}<section><h2>Human Notes</h2><div class="block">{{.HumanReasoning}}</div></section>{{end}}
  {{if .Notes}}<section><h2>Additional Notes</h2><div class="block">{{.Notes}}</div></section>{{end}}
  {{if .Warnings}}<section><h2>Warnings</h2><div class="block">{{.Warnings}}</div></section>{{end}}
</main>
</body>
</html>
