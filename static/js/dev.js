document.addEventListener('alpine:init', () => {
    Alpine.data('componentFilter', () => ({
        query: '',
        matches(name) {
            if (this.query === '') return true;
            return name.toLowerCase().includes(this.query.toLowerCase());
        },
        noResults() {
            if (this.query === '') return false;
            const cards = this.$root.querySelectorAll('[data-component]');
            for (const el of cards) {
                if (this.matches(el.dataset.component)) return false;
            }
            return true;
        },
        clear() {
            this.query = '';
        },
    }));
});
