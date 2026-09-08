import { asList, asRecord, asText, humanize } from "@/lib/scientific";

export function MeasurementContract({
  evaluation,
}: {
  evaluation: Record<string, unknown>;
}) {
  const measurements = asList(evaluation.measurements).map(asRecord);
  if (!measurements.length) return null;
  const rationale = asRecord(evaluation.rationale);
  const timing = asRecord(evaluation.measurement);
  return (
    <section className="content-section">
      <h3>What the measurements establish</h3>
      <p>{asText(rationale.improvementMeaning)}</p>
      <div className="table-scroll">
        <table className="data-table">
          <thead>
            <tr>
              <th>Measurement</th>
              <th>Purpose</th>
              <th>Meaning</th>
            </tr>
          </thead>
          <tbody>
            {measurements.map((m) => (
              <tr key={asText(m.name)}>
                <td>
                  <code>{asText(m.name)}</code>
                  <br />
                  <small>{asText(m.unit)}</small>
                </td>
                <td>
                  {humanize(asText(m.role))}
                  <br />
                  <small>{humanize(asText(m.interpretation))}</small>
                </td>
                <td>{asText(m.definition)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <p>
        <strong>Claim scope.</strong> {asText(rationale.permittedClaim)}
      </p>
      {asList(rationale.excludedClaims).length > 0 && (
        <ul>
          {asList(rationale.excludedClaims).map((v, i) => (
            <li key={i}>{asText(v)}</li>
          ))}
        </ul>
      )}
      {timing.estimator ? (
        <details>
          <summary>Timing and uncertainty</summary>
          <p>{asText(timing.timerBoundary)}</p>
          <p>{asText(timing.population)}</p>
          <dl className="contract-grid">
            <div>
              <dt>Paired trials</dt>
              <dd>{asText(timing.repetitions)}</dd>
            </div>
            <div>
              <dt>Warmup pairs</dt>
              <dd>{asText(timing.warmups)}</dd>
            </div>
            <div>
              <dt>Maximum relative interval width</dt>
              <dd>{asText(timing.maxRelativeWidth)}</dd>
            </div>
          </dl>
          <p>
            Ranking uses the lower confidence bound from broker-measured paired
            runs. Quality checks must pass. Excessive uncertainty produces an
            inconclusive result.
          </p>
        </details>
      ) : null}
    </section>
  );
}
