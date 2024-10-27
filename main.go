package main

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/bwmarrin/discordgo"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

var (
	discordToken = os.Getenv("DISCORD_BOT_TOKEN")
	channelID    = os.Getenv("DISCORD_CHANNEL_ID")
	amazonURLs   = []string{
		"https://a.co/d/dLgkilE",
		"https://a.co/d/d6vEEXI",
		"https://a.co/d/7tC83zP",
		"https://a.co/d/5lRIZfk",
		"https://a.co/d/2o5KBYB",
		"https://a.co/d/1U0OQkb",
		"https://a.co/d/0NvP60s",
	}
)

func main() {
	if discordToken == "" || channelID == "" {
		log.Fatal("DISCORD_BOT_TOKEN and DISCORD_CHANNEL_ID must be set")
	}

	discord, err := discordgo.New("Bot " + discordToken)
	if err != nil {
		log.Fatalf("Error creating Discord session: %v", err)
	}

	err = discord.Open()
	if err != nil {
		log.Fatalf("Error opening Discord connection: %v", err)
	}
	defer discord.Close()

	checkAllProducts(discord)
}

func checkAllProducts(discord *discordgo.Session) {
	for _, url := range amazonURLs {
		log.Printf("Checking URL: %s", url)
		available, status, err := checkAvailability(url)
		if err != nil {
			log.Printf("Error checking %s: %v", url, err)
			continue
		}

		log.Printf("Result for %s: Available=%v, Status=%s", url, available, status)

		// Create a shorter URL for the message
		shortURL := url
		if idx := strings.Index(url, "/ref="); idx != -1 {
			shortURL = url[:idx]
		}
		if idx := strings.Index(shortURL, "?"); idx != -1 {
			shortURL = shortURL[:idx]
		}

		if available {
			availableMessage := fmt.Sprintf("🎉 This item is now available! Check it out: %s", shortURL)
			_, err = discord.ChannelMessageSend(channelID, availableMessage)
			if err != nil {
				log.Printf("Error sending availability message to Discord: %v", err)
			}
		}

		// Sleep between checks to avoid rate limiting
		time.Sleep(10 * time.Second)
	}
}

func checkAvailability(url string) (bool, string, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, "", err
	}

	// More realistic headers
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Connection", "keep-alive")

	resp, err := client.Do(req)
	if err != nil {
		return false, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Sprintf("HTTP Status: %d", resp.StatusCode), nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, "", err
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return false, "", err
	}

	// Log the full page content for debugging
	// log.Printf("Page content: %s", doc.Find("body").Text())

	// Check for buy button (primary indicator of availability)
	buyButton := doc.Find("#add-to-cart-button, #buy-now-button").Length() > 0
	log.Printf("Buy button present: %v", buyButton)

	// Check for price (multiple possible selectors)
	priceElement := doc.Find("#price_inside_buybox, #priceblock_ourprice, .a-price .a-offscreen, #corePrice_feature_div .a-price .a-offscreen").First()
	price := strings.TrimSpace(priceElement.Text())
	hasPrice := price != ""
	log.Printf("Has price: %v, Price: %s", hasPrice, price)

	// Check for unavailability indicators
	unavailableIndicators := []string{
		"currently unavailable",
		"this item is not available",
		"we don't know when or if this item will be back in stock",
		"sign up to be notified when this item becomes available",
		"temporarily out of stock",
	}

	// Get availability section text
	availabilitySection := doc.Find("#availability, #outOfStock, #availability_feature_div").Text()
	availabilityText := strings.ToLower(availabilitySection)
	log.Printf("Availability section text: %s", availabilityText)

	// Check if any unavailability indicators are present in the availability section
	for _, indicator := range unavailableIndicators {
		if strings.Contains(availabilityText, indicator) {
			log.Printf("Found unavailability indicator: %s", indicator)
			return false, "Currently unavailable", nil
		}
	}

	// Check for in stock text
	inStockIndicators := []string{
		"in stock",
		"ships from",
		"fulfilled by amazon",
	}

	for _, indicator := range inStockIndicators {
		if strings.Contains(availabilityText, indicator) {
			log.Printf("Found in-stock indicator: %s", indicator)
			if price != "" {
				return true, fmt.Sprintf("Available - Price: %s", price), nil
			}
			return true, "Available", nil
		}
	}

	// If we have both a buy button and a price, consider it available
	if buyButton && hasPrice {
		if price != "" {
			return true, fmt.Sprintf("Available - Price: %s", price), nil
		}
		return true, "Available", nil
	}

	// If we reach here, we couldn't definitively determine availability
	log.Println("Could not definitively determine availability - defaulting to unavailable")
	return false, "Status unclear - possibly unavailable", nil
}
