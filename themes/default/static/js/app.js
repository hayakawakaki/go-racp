document.addEventListener('alpine:init', () => {
    Alpine.data('serverClock', () => ({
        now: '',
        init() {
            const tz = this.$el.dataset.tz || 'UTC';
            const fmt = new Intl.DateTimeFormat('en-US', { timeZone: tz, hour: '2-digit', minute: '2-digit', hour12: false });
            const update = () => { this.now = fmt.format(new Date()); };
            update();
            setInterval(update, 60000);
        },
    }));

    Alpine.data('adminDashboard', () => ({
        online: {},
        general: {},
        _onlineInterval: null,
        _generalInterval: null,
        init() {
            let seed = {};
            try { seed = JSON.parse(this.$el.dataset.seed || '{}'); } catch (e) { seed = {}; }
            this.online = seed.online || {};
            this.general = seed.general || {};
            const onlineMs = Number(this.$el.dataset.onlineMs) || 60000;
            const generalMs = Number(this.$el.dataset.generalMs) || 3600000;
            const onlineTick = () => {
                fetch('/api/v1/metrics/online').then(r => r.json()).then(o => { this.online = o; }).catch(() => {});
            };
            const generalTick = () => {
                fetch('/api/v1/metrics/general').then(r => r.json()).then(g => { this.general = g; }).catch(() => {});
            };
            this._onlineInterval = setInterval(onlineTick, onlineMs);
            this._generalInterval = setInterval(generalTick, generalMs);
        },
        destroy() {
            if (this._onlineInterval !== null) clearInterval(this._onlineInterval);
            if (this._generalInterval !== null) clearInterval(this._generalInterval);
        },
    }));

    Alpine.magic('copy', (el) => (text, holdMs) => {
        if (!navigator.clipboard) return;
        navigator.clipboard.writeText(text).then(() => {
            el.dispatchEvent(new CustomEvent('copied', { bubbles: false }));
            setTimeout(() => {
                el.dispatchEvent(new CustomEvent('copy-reset', { bubbles: false }));
            }, Number(holdMs) || 1500);
        }).catch(() => {});
    });
});
