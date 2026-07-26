<script setup>
function openNodeDocs() {
  window.open('/NODES.md', '_blank', 'noopener,noreferrer');
}
</script>

<template>
  <div class="page-stack">
    <section class="page-toolbar">
      <div>
        <h2>Help</h2>
        <p>Operational references for building, running, and debugging workflows.</p>
      </div>
    </section>

    <section class="workflow-grid">
      <article class="workflow-card">
        <div class="card-main">
          <h3>Node guide</h3>
          <p>Open the bundled node documentation for parameters, credentials, and examples.</p>
        </div>
        <button class="btn btn-primary" type="button" @click="openNodeDocs">Open docs</button>
      </article>
      <article class="workflow-card">
        <div class="card-main">
          <h3>First workflow checklist</h3>
          <p>Add a trigger, add an action, configure required fields, connect nodes, test, inspect output, then activate.</p>
        </div>
      </article>
      <article class="workflow-card">
        <div class="card-main">
          <h3>Troubleshooting</h3>
          <p>If the API is unavailable, check the server process and API key in Settings. If live updates disconnect, workflow runs still use REST API as source of truth.</p>
        </div>
      </article>
    </section>
  </div>
</template>
