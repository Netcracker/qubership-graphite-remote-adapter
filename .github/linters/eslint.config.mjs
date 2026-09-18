// See for details:https://eslint.org/docs/latest/use/configure/configuration-files#configuration-file

import jsonc from "eslint-plugin-jsonc";

export default [
    {
        // Note: there should be no other properties in this object
        ignores: ["**/ui/static/js/bootstrap.min.js", "**/ui/static/js/jquery.js"],
    },
    ...jsonc.configs["flat/recommended-with-json"].map((config) => ({
        ...config,
        files: ["**/*.json"],
    })),
];
