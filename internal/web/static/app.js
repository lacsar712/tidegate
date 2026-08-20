async function fetchStatus() {
  const resp = await fetch('/v1/status');
  if (!resp.ok) {
    throw new Error('status request failed');
  }
  return resp.json();
}

function renderChambers(snapshot) {
  const root = document.getElementById('chambers');
  root.innerHTML = '';
  for (const ch of snapshot.chambers || []) {
    const card = document.createElement('article');
    card.className = 'chamber-card';
    card.innerHTML = `
      <h3>${ch.name} <small>(${ch.id})</small></h3>
      <div class="levels">
        <span>Upstream: ${ch.smoothedUpstreamCm ?? '-'} cm</span>
        <span>Downstream: ${ch.smoothedDownstreamCm ?? '-'} cm</span>
        <span>Head diff: ${ch.headDiffCm ?? '-'} cm</span>
      </div>
    `;
    for (const gate of ch.gates || []) {
      const row = document.createElement('div');
      row.className = 'gate-row';
      row.innerHTML = `
        <strong>${gate.name}</strong>
        <span class="state-${gate.state}">${gate.state} @ ${gate.openPercent}%</span>
      `;
      card.appendChild(row);
    }
    root.appendChild(card);
  }
}

function renderDenials(snapshot) {
  const list = document.getElementById('deny-list');
  list.innerHTML = '';
  const items = snapshot.recentDeny || [];
  if (items.length === 0) {
    const li = document.createElement('li');
    li.textContent = 'No recent denials';
    list.appendChild(li);
    return;
  }
  for (const d of items) {
    const li = document.createElement('li');
    li.textContent = `[${d.code}] ${d.message}`;
    list.appendChild(li);
  }
}

async function refresh() {
  try {
    const snapshot = await fetchStatus();
    renderChambers(snapshot);
    renderDenials(snapshot);
    document.getElementById('generated-at').textContent =
      'Updated ' + new Date(snapshot.generatedAt).toLocaleString();
  } catch (err) {
    document.getElementById('generated-at').textContent = 'Refresh failed: ' + err.message;
  }
}

document.getElementById('open-form').addEventListener('submit', async (ev) => {
  ev.preventDefault();
  const payload = {
    chamberId: document.getElementById('chamber-id').value,
    gateId: document.getElementById('gate-id').value,
    action: document.getElementById('action').value,
  };
  const resp = await fetch('/v1/ops/request-open', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  const body = await resp.json();
  document.getElementById('action-result').textContent = JSON.stringify(body, null, 2);
  await refresh();
});

refresh();
setInterval(refresh, 5000);
