export async function updateNav() {
    const elem = document.getElementById('nav');
    const content = `
        <div class="navbar-brand">
            <a class="navbar-item" href="/index.html">
                <h1 class="is-size-2">DIX<sup>10</sup></h1>
                <p>A block explorer<br>
                for Polkadot!</p>
            </a>
            <a role="button" class="navbar-burger" aria-label="menu" aria-expanded="false" data-target="navbarMenu">
                <span aria-hidden="true"></span>
                <span aria-hidden="true"></span>
                <span aria-hidden="true"></span>
                <span aria-hidden="true"></span>
            </a>
        </div>

        <div id="navbarMenu" class="navbar-menu">
            <div class="navbar-start">
                <a class="navbar-item is-size-4" href="/balances.html">Balances</a>
                <a class="navbar-item is-size-4" href="/staking.html">Staking</a>
                <a class="navbar-item is-size-4" href="/blocks.html">Blocks</a>
                <a class="navbar-item is-size-4" href="/stats.html">Statistics</a>
            </div>
            <div class="navbar-end">
                <a class="navbar-item is-size-4" href="https://github.com/pierreaubert/dotidx" target="_blank" rel="noopener noreferrer">GitHub</a>
                <a class="navbar-item is-size-4" href="https://github.com/pierreaubert/dotidx/issues/new" target="_blank" rel="noopener noreferrer">Report a bug</a>
                <a class="navbar-item is-size-4" href="https://github.com/pierreaubert/dotidx/blob/main/LICENSE" target="_blank" rel="noopener noreferrer">License</a>
            </div>
        </div>
`;
    elem.innerHTML = content;

    // Get all "navbar-burger" elements
    const $navbarBurgers = Array.prototype.slice.call(document.querySelectorAll('.navbar-burger'), 0);

    // Add a click event on each of them
    $navbarBurgers.forEach((el) => {
        el.addEventListener('click', () => {
            const target = el.dataset.target;
            const $target = document.getElementById(target);

            // Toggle the "is-active" class on both the "navbar-burger" and the "navbar-menu"
            el.classList.toggle('is-active');
            $target.classList.toggle('is-active');
        });
    });
}

export async function updateFooter() {
    const elem = document.getElementById('footer');
    const content = `
<div class="content has-text-centered">
   <p><strong>DIX</strong> - An open source Polkadot Block Explorer</p>
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
            <button id="action-button" class="button is-link">Go</button>
          </div>
        </div>

        <div class="columns mt-3">
          <div class="column">
            <div class="field">
              <label class="label">Count</label>
              <div class="control">
                <input id="search-count"
                       class="input"
                       type="number"
                       value="20" min="1" max="100"
                       title="Number of records to display">
              </div>
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
            <div class="select">
              <select id="search-block-relaychain">
                <option>Select a relay chain</option>
                <option value="polkadot"selected>Polkadot</option>
                <option value="kusama">Kusama</option>
              </select>
            </div>
          </div>
          <div class="control">
            <div class="select">
              <select id="search-block-chain">
                <option>Select a chain</option>
                <option value="polkadot"selected>Polkadot</option>
                <option value="assethub">AssetHub</option>
                <option value="people">People</option>
                <option value="collectives">Collectives</option>
                <option value="frequency">Frequency</option>
                <option value="mythos">Mythical</option>
              </select>
            </div>
          </div>
          <div class="control">
            <button id="action-button" class="button is-warning">Go</button>
          </div>
        </div>
`;
    elem.innerHTML = content;
}
