document.addEventListener('alpine:init', () => {
    Alpine.data('modal', () => ({
        open: false,
        init() {
            this.open = this.$el.dataset.modalOpen === 'true';
        },
        show() {
            this.open = true;
        },
        hide() {
            this.open = false;
        },
        lockScroll() {
            document.body.style.overflow = this.open ? 'hidden' : '';
        },
    }));

    Alpine.data('toastContainer', () => ({
        toasts: [],
        add(detail) {
            if (!detail || !detail.message) return;
            const id = Date.now() + Math.random();
            this.toasts.push({
                id,
                type: detail.type || 'info',
                size: detail.size || 'md',
                message: detail.message,
            });
            const duration = Number(detail.duration) || 4000;
            setTimeout(() => this.remove(id), duration);
        },
        remove(id) {
            this.toasts = this.toasts.filter(t => t.id !== id);
        },
        toastClass(type, size) {
            const variants = {
                success: 'bg-green-50 border-green-200 text-green-900',
                error: 'bg-red-50 border-red-200 text-red-900',
                warning: 'bg-yellow-50 border-yellow-200 text-yellow-900',
                info: 'bg-blue-50 border-blue-200 text-blue-900',
            };
            const sizes = {
                sm: 'min-w-[220px] max-w-xs px-3 py-2 text-xs gap-2',
                md: 'min-w-[280px] max-w-sm px-4 py-3 text-sm gap-3',
                lg: 'min-w-[360px] max-w-md px-5 py-4 text-base gap-3',
            };
            const variant = variants[type] || 'bg-white border-slate-200 text-slate-900 dark:bg-slate-900 dark:border-slate-700 dark:text-slate-100';
            const sizing = sizes[size] || sizes.md;
            return 'pointer-events-auto rounded-lg border shadow-md flex items-start ' + sizing + ' ' + variant;
        },
    }));

    Alpine.data('pagination', () => ({
        max: 1,
        pattern: '?page=__PAGE__',
        init() {
            this.max = parseInt(this.$el.dataset.paginationMax, 10) || 1;
            this.pattern = this.$el.dataset.paginationPattern || this.pattern;
        },
        go(value) {
            let page = parseInt(value, 10);
            if (!page || page < 1) page = 1;
            if (page > this.max) page = this.max;
            window.location.href = this.pattern.replace('__PAGE__', String(page));
        },
    }));

    Alpine.data('dropdown', () => ({
        open: false,
        toggle() {
            this.open = !this.open;
        },
        show() {
            this.open = true;
        },
        close() {
            this.open = false;
        },
    }));

    Alpine.data('accordionItem', () => ({
        open: false,
        init() {
            this.open = this.$el.dataset.state === 'open';
        },
        toggle() {
            this.open = !this.open;
        },
        show() {
            this.open = true;
        },
        close() {
            this.open = false;
        },
        stateAttr() {
            return this.open ? 'open' : 'closed';
        },
    }));

    Alpine.magic('toast', () => {
        return (type, message, duration, size) => {
            window.dispatchEvent(new CustomEvent('toast', {
                detail: { type, message, duration, size },
            }));
        };
    });
});

document.addEventListener('click', e => {
    const trigger = e.target.closest('[data-toast]');
    if (!trigger) return;
    window.dispatchEvent(new CustomEvent('toast', {
        detail: {
            type: trigger.dataset.toastType || 'info',
            size: trigger.dataset.toastSize || 'md',
            message: trigger.dataset.toast,
            duration: Number(trigger.dataset.toastDuration) || undefined,
        },
    }));
});
