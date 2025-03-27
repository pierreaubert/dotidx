export function insertSearchAddressBox(inDiv) {
    const html = `
<div class="box mb-5">
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
    inDiv.innerHTML = html;
}
