export default class Adapter {
    constructor({ post, login, readLoginFromCache }) {
        this._post = post;
        this._login = login;
    }

    async post(content, image) {
        return this._post(content, image);
    }

    async login(username, password) {
        return this._login(username, password)
    }
}