module.exports = {
    entry: './src/main.ts',
    output: {
        path: __dirname + "/dist",
        filename: './js/index.js',
    },
    resolve: {
        extensions: ['.webpack.js', '.web.js', '.ts', '.tsx', '.js']
    },
    module: {
        rules: [
            {
                test: /\.tsx?$/,
                use: ['ts-loader']
            }
        ]
    },
    plugins: [
    ]
};
