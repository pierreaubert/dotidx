import { updateHero, updateFooter, updateNav } from './components.js';

document.addEventListener('DOMContentLoaded', () => {
    initApp();
});

async function initApp() {
    // Initialize the app
    async function init() {
        await updateHero();
        await updateNav();
        await updateFooter();
    }

    // Call the initialization function
    await init();
}
