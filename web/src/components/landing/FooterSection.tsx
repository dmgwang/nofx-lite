import { Language } from '../../i18n/translations'

interface FooterSectionProps {
    language: Language
}

export default function FooterSection({ }: FooterSectionProps) {
    return (
        <footer
            style={{
                borderTop: '1px solid var(--panel-border)',
                background: 'var(--brand-dark-gray)',
            }}
        >
            <div
                className="container py-4 flex justify-center items-center mx-auto px-4 text-sm text-gray-400 sm:px-6 lg:px-8"
            >Nofx Lite Forked from Nofx</div>
        </footer>
    )
}
