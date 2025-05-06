const purgecss = require('@fullhuman/postcss-purgecss')
const cssnano = require('cssnano')
const pruneVar = require('postcss-prune-var')
const varCompress = require('postcss-variable-compress')

module.exports = {
    plugins: [
        purgecss({
            // file paths to your contents to remove unused styles.
            content: ['app/*.js', 'app/*.html'],
            // other wise our aria-selected is removed (like with purgecss online)
            dynamicAttributes: ["aria-selected"],
        }),
        pruneVar(), // remove unused css variables
        varCompress(), // compress css variables
        cssnano({
            preset: 'default',
        }),
    ],
};
