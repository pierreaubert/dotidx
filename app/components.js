export async function updateNav() {
    const elem = document.getElementById('nav');
    const content = `
        <div class="navbar-brand">
            <a class="navbar-item" href="/">
                <h1 class="title has-text-white">DIX a polka<strong>D</strong>ot <strong>I</strong>nde<strong>X</strong>er<h1>
            </a>
            <a role="button" class="navbar-burger" aria-label="menu" aria-expanded="false" data-target="navbarMenu">
                <span aria-hidden="true"></span>
                <span aria-hidden="true"></span>
                <span aria-hidden="true"></span>
            </a>
        </div>

        <div id="navbarMenu" class="navbar-menu">
            <div class="navbar-end">
                <a class="navbar-item" href="/balances.html">Balances</a>
                <a class="navbar-item" href="/staking.html">Staking</a>
                <a class="navbar-item" href="/blocks.html">Blocks</a>
                <a class="navbar-item" href="/stats.html">Statistics</a>
                <a class="navbar-item" href="https://github.com/pierreaubert/dotidx" target="_blank" rel="noopener noreferrer">GitHub</a>
            </div>
        </div>
`;
    elem.innerHTML = content;
}

export async function updateHero() {
    const elem = document.getElementById('hero');
    const content = `
        <div class="hero-body">
            <p class="title">A Polkadot Blockchain Data Explorer</p>
            <p class="subtitle">Explore balances, staking or blocks per address, accross chains and some statistics!</p>
        </div>
    </section>
`;
    elem.innerHTML = content;
}

export async function updateFooter() {
    const elem = document.getElementById('footer');
    const content = `
<div class="content has-text-centered">
   <p><strong>DIX</strong> - A Polkadot Blockchain Data Explorer</p>
</div>
`;
    elem.innerHTML = content;
}

export async function updateSearchAssets(target) {
    const elem = document.getElementById(target);
    const content = `
        <div class="field has-addons">
          <div class="control is-expanded has-icons-left">
            <input id="search-address" class="input" type="text" placeholder="Enter address">
            <span class="icon is-small is-left">
              <i class="fas fa-search"></i>
            </span>
          </div>
          <div class="control">
            <button id="action-button" class="button is-primary">Go</button>
          </div>
        </div>

        <div class="columns mt-3">
          <div class="column">
            <div class="field">
              <label class="label">Count</label>
              <div class="control">
                <input id="search-count" class="input" type="number" value="20" min="1" max="100" title="Number of records to display">
                            </div>
                            <p class="help">Number of records to display</p>
            </div>
          </div>
          <div class="column">
            <div class="field">
              <label class="label">From Date</label>
              <div class="control">
                <input id="search-from" class="input" type="datetime-local" title="Start date for filtering" placeholder="Select start date">
                            </div>
            </div>
          </div>
          <div class="column">
            <div class="field">
              <label class="label">To Date</label>
              <div class="control">
                <input id="search-to" class="input" type="datetime-local" title="End date for filtering" placeholder="Select end date">
              </div>
            </div>
          </div>
          <div class="column is-narrow">
            <div class="field">
              <label class="label">&nbsp;</label>
              <div class="control buttons">
                <button id="apply-filters" class="button is-info">Apply Filters</button>
                <button id="clear-filters" class="button is-light">Clear Filters</button>
              </div>
            </div>
          </div>
        </div>
      </div>
`;
    elem.innerHTML = content;
}

export async function updateSearchBlocks() {
    const elem = document.getElementById('search-blocks');
    const content = `
        <div class="field has-addons">
          <div class="control is-expanded has-icons-left">
            <input id="search-block" class="input" type="text" placeholder="Enter a blockID">
            <span class="icon is-small is-left">
              <i class="fas fa-search"></i>
            </span>
          </div>
          <div class="control">
            <button id="action-button" class="button is-primary">Go</button>
          </div>
        </div>
`;
    elem.innerHTML = content;
}
