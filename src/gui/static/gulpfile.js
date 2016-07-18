"use strict";

const gulp = require("gulp");
const del = require("del");
const tsc = require("gulp-typescript");
const sourcemaps = require('gulp-sourcemaps');
var concat = require('gulp-concat');
var uglify = require('gulp-uglify');
const tsProject = tsc.createProject("tsconfig.json");
var replace = require('gulp-replace');

/**
 * Remove build directory.
 */
gulp.task('clean', (cb) => {
    return del(["dist"], cb);
});

/**
 * Compile TypeScript sources and create sourcemaps in build directory.
 */
gulp.task("compile", () => {
    let tsResult = gulp.src("src/**/*.ts")
        .pipe(sourcemaps.init())
        .pipe(tsc(tsProject));
    return tsResult.js
        .pipe(replace('.ts', '.js'))
        .pipe(sourcemaps.write("."))
        .pipe(gulp.dest("dist"));
});

/**
 * Copy all resources that are not TypeScript files into build directory.
 */
gulp.task("resources", () => {
    return gulp.src(["src/**/*", "!**/*.ts"])
        .pipe(gulp.dest("dist"));
});

/**
 * Build the project.
 */
gulp.task("build", ['compile', 'resources'], () => {
    console.log("Building the project ...");
});