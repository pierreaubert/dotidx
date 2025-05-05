import { resolve } from 'path';
import { defineConfig } from 'vite';

export default defineConfig({
    // Set the root directory to 'app' where your HTML files are
    root: resolve(__dirname, 'app'),
    build: {
	outDir: resolve(__dirname, 'app/dist'),
	emptyOutDir: true,
	rollupOptions: {
	    input: {
		main: resolve(__dirname, './app/index.html'),
		balances: resolve(__dirname, 'app/balances.html'),
		blocks: resolve(__dirname, 'app/blocks.html'),
		staking: resolve(__dirname, 'app/staking.html'),
		stats: resolve(__dirname, 'app/stats.html'),
		maintenance: resolve(__dirname, 'app/maintenance.html'),
	    },
	},
    },
    server: {
	open: '/index.html',
    },
});
