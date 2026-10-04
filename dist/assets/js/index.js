import { App }        from './app.js';

const pageLogin = "login";
const pageRoom = "room";
const pageLock = "lockscreen";
const pageMain = "home";

const app= new App();

window.onload = async (event) => {
    console.log("Page loaded");

    window.app = app;

    await app.init();
}