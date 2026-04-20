export default class Adapter {
    constructor({ post, login, readLoginFromCache }) {
        this._post = post;
        this._login = login;
    }

    async post(content) {
        return this._post(content);
    }

    async login(username, password) {
        return this._login(username, password)
    }
}