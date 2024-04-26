const { execSync } = require('child_process');

console.log("copy static files...");
execSync("cp -r -f ./static/* ./dist/");
console.log("generate file...");
execSync("cd ./src/blend/ && go run blend.go");
console.log("gopherjs build...");
execSync("cd ./src/psd/ && go mod download && gopherjs build -m -o /PSDTool/dist/js/psd.min.js");
console.log("webpack build...");
execSync("webpack --mode production");
