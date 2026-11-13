const ADMIN_MODEL_ROUTE_COUNT = 6;

export function AdminModelsSkeletonRows() {
  return <>
    {Array.from({ length: ADMIN_MODEL_ROUTE_COUNT }, (_, index) => (
      <section className="model-route-row" aria-hidden="true" key={index}>
        <header className="model-route-row__header"><span className="dashboard-skeleton model-route-row__skeleton-title" /></header>
        <div className="model-route-row__controls"><span className="dashboard-skeleton model-route-row__skeleton-control" /></div>
        <p className="model-route-row__details"><span className="dashboard-skeleton model-route-row__skeleton-details" /></p>
      </section>
    ))}
  </>;
}
