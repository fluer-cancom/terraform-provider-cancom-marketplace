package marketplace

type Subscription struct {
	Id                          string        `json:"id"`
	ParentSubscriptionId        *string       `json:"parentSubscriptionId"`
	CreationDate                int64         `json:"creationDate"`
	EndDate                     *int64        `json:"endDate"`
	ExternalId                  *string       `json:"externalId"`
	ExternalAccountId           string        `json:"externalAccountId"`
	Status                      string        `json:"status"`
	Label                       *string       `json:"label"`
	MaxUsers                    *int          `json:"maxUsers"`
	AssignedUsers               int           `json:"assignedUsers"`
	Order                       Order         `json:"order"`
	UpcomingOrder               *Order        `json:"upcomingOrder"`
	User                        User          `json:"user"`
	Company                     Company       `json:"company"`
	Product                     Product       `json:"product"`
	Edition                     Edition       `json:"edition"`
	RedirectUrl                 *string       `json:"redirectUrl"`
	InternalId                  string        `json:"internalId"`
	BundleApplicationId         *string       `json:"bundleApplicationId"`
	IsFreeTrialExtensionRequest bool          `json:"isFreeTrialExtensionRequest"`
	AutoRenew                   bool          `json:"autoRenew"`
	CustomAttributes            []interface{} `json:"customAttributes"`
	FreeTrialExtensionRequest   bool          `json:"freeTrialExtensionRequest"`
}

type Order struct {
	UUID                 string             `json:"uuid"`
	StartDate            int64              `json:"startDate"`
	EndDate              *int64             `json:"endDate"`
	ServiceStartDate     string             `json:"serviceStartDate"`
	NextBillingDate      int64              `json:"nextBillingDate"`
	EndOfDiscountDate    *int64             `json:"endOfDiscountDate"`
	CycleStartDate       int64              `json:"cycleStartDate"`
	BillingEndDate       *int64             `json:"billingEndDate"`
	Status               string             `json:"status"`
	Frequency            string             `json:"frequency"`
	Currency             string             `json:"currency"`
	Type                 string             `json:"type"`
	TotalPrice           float64            `json:"totalPrice"`
	User                 User               `json:"user"`
	SalesSupportUser     *User              `json:"salesSupportUser"`
	SalesSupportCompany  *Company           `json:"salesSupportCompany"`
	Company              Company            `json:"company"`
	ReferenceCode        *string            `json:"referenceCode"`
	TransactionMode      string             `json:"transactionMode"`
	PaymentPlan          PaymentPlan        `json:"paymentPlan"`
	Contract             *Contract          `json:"contract"`
	ParentSubscriptionId *string            `json:"parentSubscriptionId"`
	PreviousOrder        *Order             `json:"previousOrder"`
	NextOrder            *Order             `json:"nextOrder"`
	PaymentPlanId        int                `json:"paymentPlanId"`
	DiscountId           *string            `json:"discountId"`
	Activated            bool               `json:"activated"`
	OneTimeOrders        []interface{}      `json:"oneTimeOrders"`
	OrderLines           []OrderLine        `json:"orderLines"`
	Parameters           *[]Parameter       `json:"parameters"`
	CustomAttributes     *[]CustomAttribute `json:"customAttributes"`
	Links                *[]Link            `json:"links"`
	// The API returns order IDs as both JSON numbers and strings (for example,
	// previousOrder.id), so retain the value without imposing one representation.
	Id interface{} `json:"id"`
}

type User struct {
	Id string `json:"id"`
}

type Company struct {
	Id string `json:"id"`
}

type Product struct {
	Id string `json:"id"`
}

type Edition struct {
	Id string `json:"id"`
}

type PaymentPlan struct {
	Id                              int                  `json:"id"`
	UUID                            string               `json:"uuid"`
	Frequency                       string               `json:"frequency"`
	Status                          string               `json:"status"`
	Contract                        *PaymentPlanContract `json:"contract"`
	AllowCustomUsage                bool                 `json:"allowCustomUsage"`
	KeepBillDateOnUsageChange       bool                 `json:"keepBillDateOnUsageChange"`
	KeepBillDateOnPricingPlanChange bool                 `json:"keepBillDateOnPricingPlanChange"`
	SeparatePrepaid                 bool                 `json:"separatePrepaid"`
	IsPrimaryPrice                  bool                 `json:"isPrimaryPrice"`
	Costs                           []Cost               `json:"costs"`
	CreditOnCancellation            bool                 `json:"creditOnCancellation"`
	PrimaryPrice                    bool                 `json:"primaryPrice"`
}

type PaymentPlanContract struct {
	MinimumServiceLength                          int            `json:"minimumServiceLength"`
	CancellationPeriodLimit                       int            `json:"cancellationPeriodLimit"`
	EndOfContractGracePeriod                      *int           `json:"endOfContractGracePeriod"`
	BlockSwitchToShorterContract                  bool           `json:"blockSwitchToShorterContract"`
	BlockContractDowngrades                       bool           `json:"blockContractDowngrades"`
	BlockContractUpgrades                         bool           `json:"blockContractUpgrades"`
	KeepContractDateOnPlanChange                  bool           `json:"keepContractDateOnPlanChange"`
	KeepContractDateOnPlanChangeDifferentDuration bool           `json:"keepContractDateOnPlanChangeDifferentDuration"`
	AlignWithParentCycleStartDate                 bool           `json:"alignWithParentCycleStartDate"`
	GracePeriod                                   GracePeriod    `json:"gracePeriod"`
	TerminationFee                                TerminationFee `json:"terminationFee"`
	AutoExtensionPricingId                        int            `json:"autoExtensionPricingId"`
}

type GracePeriod struct {
	Length int    `json:"length"`
	Unit   string `json:"unit"`
}

type TerminationFee struct {
	Type            string       `json:"type"`
	Description     string       `json:"description"`
	PercentageFee   *float64     `json:"percentageFee"`
	FlatFee         *interface{} `json:"flatFee"`
	Percentage      *float64     `json:"percentage"`
	Price           *interface{} `json:"price"`
	EstimatedCost   *interface{} `json:"estimatedCost"`
	GracePeriod     *int         `json:"gracePeriod"`
	GracePeriodUnit *string      `json:"gracePeriodUnit"`
}

type Cost struct {
	Id                            int                `json:"id"`
	EditionPricingItemId          int                `json:"editionPricingItemId"`
	EditionPricingItemUuid        string             `json:"editionPricingItemUuid"`
	Status                        string             `json:"status"`
	Unit                          string             `json:"unit"`
	UnitDependency                *string            `json:"unitDependency"`
	MinUnits                      int                `json:"minUnits"`
	MaxUnits                      *int               `json:"maxUnits"`
	MeteredUsage                  bool               `json:"meteredUsage"`
	Increment                     int                `json:"increment"`
	PricePerIncrement             bool               `json:"pricePerIncrement"`
	BlockContractDecrease         bool               `json:"blockContractDecrease"`
	BlockContractIncrease         bool               `json:"blockContractIncrease"`
	BlockOriginalContractDecrease bool               `json:"blockOriginalContractDecrease"`
	CanIncreaseUnits              bool               `json:"canIncreaseUnits"`
	CanDecreaseUnits              bool               `json:"canDecreaseUnits"`
	Amount                        map[string]float64 `json:"amount"`
	PricingStrategy               string             `json:"pricingStrategy"`
}

type Contract struct {
	MinimumServiceLength                            int            `json:"minimumServiceLength"`
	EndOfContractDate                               int64          `json:"endOfContractDate"`
	GracePeriodEndDate                              int64          `json:"gracePeriodEndDate"`
	CancellationPeriodLimit                         int            `json:"cancellationPeriodLimit"`
	EndOfContractGracePeriod                        *int           `json:"endOfContractGracePeriod"`
	TerminationFee                                  TerminationFee `json:"terminationFee"`
	Renewal                                         Renewal        `json:"renewal"`
	AutoExtensionPricingUuid                        string         `json:"autoExtensionPricingUuid"`
	ContinueWithoutContract                         bool           `json:"continueWithoutContract"`
	BlockContractUpgrades                           bool           `json:"blockContractUpgrades"`
	BlockContractDowngrades                         bool           `json:"blockContractDowngrades"`
	BlockSwitchToShorterContract                    bool           `json:"blockSwitchToShorterContract"`
	KeepContractDateOnPlanChange                    bool           `json:"keepContractDateOnPlanChange"`
	KeepContractDateOnPlanChangeDifferentDuration   bool           `json:"keepContractDateOnPlanChangeDifferentDuration"`
	KeepBillDateOnPlanChangeSameContractLength      bool           `json:"keepBillDateOnPlanChangeSameContractLength"`
	KeepBillDateOnPlanChangeDifferentContractLength bool           `json:"keepBillDateOnPlanChangeDifferentContractLength"`
	ContractAvoidProrationOnUpdate                  bool           `json:"contractAvoidProrationOnUpdate"`
	AlignWithParentCycleStartDate                   bool           `json:"alignWithParentCycleStartDate"`
	UnitTerms                                       []UnitTerm     `json:"unitTerms"`
}

type Renewal struct {
	Order       *interface{} `json:"order"`
	PaymentPlan PaymentPlan  `json:"paymentPlan"`
}

type UnitTerm struct {
	Unit                          string `json:"unit"`
	BlockContractIncrease         bool   `json:"blockContractIncrease"`
	BlockOriginalContractDecrease bool   `json:"blockOriginalContractDecrease"`
	BlockContractDecrease         bool   `json:"blockContractDecrease"`
}

type OrderLine struct {
	Id                   int      `json:"id"`
	EditionPricingItemId *int     `json:"editionPricingItemId"`
	Type                 string   `json:"type"`
	Unit                 *string  `json:"unit"`
	Quantity             int      `json:"quantity"`
	Price                float64  `json:"price"`
	ListingPrice         *float64 `json:"listingPrice"`
	TotalPrice           float64  `json:"totalPrice"`
	ApplicationName      string   `json:"applicationName"`
	EditionName          *string  `json:"editionName"`
	Description          string   `json:"description"`
	Percentage           *float64 `json:"percentage"`
}

type Parameter struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type CustomAttribute struct {
	Name          string   `json:"name"`
	AttributeType string   `json:"attributeType"`
	Value         string   `json:"value"`
	ValueKeys     []string `json:"valueKeys"`
}

type Link struct {
	Rel string `json:"rel"`
}
