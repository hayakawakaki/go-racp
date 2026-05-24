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
});
