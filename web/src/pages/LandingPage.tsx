import { useState } from 'react'
import HeaderBar from '../components/landing/HeaderBar'
import AboutSection from '../components/landing/AboutSection'
import FeaturesSection from '../components/landing/FeaturesSection'
import HowItWorksSection from '../components/landing/HowItWorksSection'
import LoginModal from '../components/landing/LoginModal'
import FooterSection from '../components/landing/FooterSection'
import { useAuth } from '../contexts/AuthContext'
import { useLanguage } from '../contexts/LanguageContext'

export function LandingPage() {
    const [showLoginModal, setShowLoginModal] = useState(false)
    const { user, logout } = useAuth()
    const { language, setLanguage } = useLanguage()
    const isLoggedIn = !!user

    console.log('LandingPage - user:', user, 'isLoggedIn:', isLoggedIn)
    return (
        <>
            <HeaderBar
                onLoginClick={() => setShowLoginModal(true)}
                isLoggedIn={isLoggedIn}
                isHomePage={true}
                language={language}
                onLanguageChange={setLanguage}
                user={user}
                onLogout={logout}
                onPageChange={(page) => {
                    console.log('LandingPage onPageChange called with:', page)
                    if (page === 'competition') {
                        window.location.href = '/competition'
                    } else if (page === 'traders') {
                        window.location.href = '/traders'
                    } else if (page === 'trader') {
                        window.location.href = '/dashboard'
                    }
                }}
            />
            <div
                className="min-h-screen px-4 py-12 sm:px-6 lg:px-8"
                style={{
                    background: 'var(--brand-black)',
                    color: 'var(--brand-light-gray)',
                }}
            >
                <AboutSection language={language} />
                <FeaturesSection language={language} />
                <HowItWorksSection language={language} />

                {showLoginModal && (
                    <LoginModal
                        onClose={() => setShowLoginModal(false)}
                        language={language}
                    />
                )}
                <FooterSection language={language} />
            </div>
        </>
    )
}
