import i18n from 'i18next'
import {initReactI18next} from 'react-i18next'
import fr from './fr.json'

// Français par défaut ; l'anglais s'ajoutera en déposant en.json (spec §2).
void i18n.use(initReactI18next).init({
    resources: {fr: {translation: fr}},
    lng: 'fr',
    fallbackLng: 'fr',
    interpolation: {escapeValue: false},
})

export default i18n
