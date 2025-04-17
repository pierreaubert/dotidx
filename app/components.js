export async function updateIcons() {
    let icons = '';

    const bars = `
<symbol id="icon-bars" viewBox="0 0 448 512">
    <path d="M0 96C0 78.3 14.3 64 32 64H416c17.7 0 32 14.3 32 32s-14.3 32-32 32H32C14.3 128 0 113.7 0 96zM0 256c0-17.7 14.3-32 32-32H416c17.7 0 32 14.3 32 32s-14.3 32-32 32H32c-17.7 0-32-14.3-32-32zM448 416c0 17.7-14.3 32-32 32H32c-17.7 0-32-14.3-32-32s14.3-32 32-32H416c17.7 0 32 14.3 32 32z"/>
  </symbol>
`;
    icons += bars;

    document.getElementById('icons').innerHTML = icons;
}

export async function updateNav() {
    const url = new URL(window.location.href);
    const path = url.pathname;
    const nicePath = path[1].toUpperCase() + path.slice(2, path.length - 5);
    let contentNav = `
          <div class="level-left">
            <a class="level-item" href="/index.html">
               <img src="/dix.svg" alt="DIX" width="48" height="48" />
`;
    if (path === '/' || path === '/index.html') {
        contentNav += `
	      A Polkadot<br/>
	      Block explorer
`;
    } else {
        contentNav += `${nicePath}`;
    }

    contentNav += `
            </a>
          </div>
          <div id="level-right">
            <div class="level-item">
              <div class="dropdown is-hoverable is-right">
                <div class="dropdown-trigger">
                  <p class="is-warning is-outlined is-size-4" aria-haspopup="true" aria-controls="dropdown-menu">
                     <span class="icon is-left">
      	               <svg width="24px" height="24px"><use href="#icon-bars"/></svg>
                     </span>
                  </p>
                </div>
                <div class="dropdown-menu" id="dropdown-menu" role="menu">
                  <div class="dropdown-content">
                    <a href="/balances.html" class="dropdown-item"> Balances </a>
                    <a href="/staking.html" class="dropdown-item"> Staking </a>
                    <hr class="dropdown-divider" />
                    <a href="/blocks.html" class="dropdown-item"> Blocks </a>
                    <a href="/stats.html" class="dropdown-item"> Statistics </a>
                    <hr class="dropdown-divider" />
                    <a class="dropdown-item" href="https://github.com/pierreaubert/dotidx">GitHub</a>
                    <a class="dropdown-item" href="https://github.com/pierreaubert/dotidx/issues/new">Report a bug</a>
                    <a class="dropdown-item" href="https://github.com/pierreaubert/dotidx/blob/main/LICENSE">License</a>
                </div>
              </div>
            </div>
          </div>
`;
    document.getElementById('nav').innerHTML = contentNav;
}

export async function updateFooter() {
    const elem = document.getElementById('footer');
    const content = `
<div class="content has-text-centered">
   <p><strong>DIX</strong> - An open source Block Explorer for Polkadot</p>
</div>
`;
    elem.innerHTML = content;
}

export async function updateModals() {
    const elem = document.getElementById('modal-add-address');
    const content = `
    <div class="modal-background">
    </div>
    <div class="modal-content">
      <div class="box">
        <div class="notification is-danger is-hidden" id="modal-alert">
          <button class="delete"></button>
            Address does not look correct!
        </div>
	<div class="field">
          <div class="control is-expanded">
	    <label class="label">Polkadot or Ethereum address</label>
            <input id="add-address" class="input" type="text" placeholder="Enter address">
          </div>
	</div>
	<div class="field is-grouped is-grouped-right">
          <div class="control">
            <button id="add-address-add-button" class="button is-link">Add</button>
          </div>
          <div class="control">
            <button id="add-address-cancel-button" class="button is-link is-light">Cancel</button>
          </div>
	</div>
      </div>
    </div>

`;
    elem.innerHTML = content;
}

export async function updateSearchAssets(target) {
    let now = new Date();
    now.setMonth(now.getMonth() - 3);
    const date = now.toISOString();
    const dot = date.indexOf('.');
    let valid = date;
    if (dot !== -1) {
        valid = date.slice(0, dot);
    }
    const elem = document.getElementById(target);
    const content = `
      <div class="box">
        <div class="field has-addons has-addons-centered">
          <p class="control">
            <button id="polkadot-connect-button" class="button">
              <span class="icon">
                <img src="https://polkadot.js.org/docs/img/logo.svg"></img>
              </span>
            </button>
          </p>
          <p class="control is-expanded">
            <span class="select is-fullwidth">
              <select id="addresses">
                <option value="">Select an address</option>
              </select>
            </span>
          </p>
          <p class="control">
            <button id="action-button" class="button is-link">Go</button>
          </p>
        </div>

        <div class="columns mt-3 is-centered">

          <div class="column is-narrow">
            <div class="field">
              <label class="label">From Date</label>
              <div class="control">
                <input id="search-from"
                       class="input"
                       type="datetime-local"
                       title="Start date for filtering"
                       placeholder="Select start date"
                       value="${valid}" />
              </div>
            </div>
          </div>
          <div class="column is-narrow">
            <div class="field">
              <label class="label">To Date</label>
              <div class="control">
                <input id="search-to"
                       class="input"
                       type="datetime-local"
                       title="End date for filtering"
                       placeholder="Select end date"
                />
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
    </div>
`;
    elem.innerHTML = content;
}

export async function updateSearchBlocks() {
    const url = new URL(window.location.href);
    const relay = url.searchParams.get('relay');
    const chain = url.searchParams.get('chain');
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
